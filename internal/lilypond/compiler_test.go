//go:build darwin || linux

package lilypond

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCompiler(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lilypond")
	script := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in --output=*) output=${arg#--output=};; esac
  source=$arg
done
printf '%s\n' "$PWD" > "$source.cwd"
printf '%s\n' "$@" > "$source.args"
mode=$(head -n 1 "$source")
case "$mode" in
  slow*) printf '%%PDF-1.4 stale' > "$output.pdf"; echo started > "$source.started"; sleep 5;;
  fail*) printf '%s:12:3: error: invalid note\n  continued detail\n' "$source" >&2; exit 1;;
  none*) exit 0;;
  loud*) dd if=/dev/zero bs=1024 count=300 2>/dev/null;;
esac
case "$mode" in
  midi*) ;;
  *) printf '%%PDF-1.4 %s' "$mode" > "$output.pdf";;
esac
case "$mode" in
  no-midi*) ;;
  *) printf 'MThd%s' "$mode" > "$output.midi";;
esac
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeScore(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCompilerArtifactsDiagnosticsAndLimits(t *testing.T) {
	executable := fakeCompiler(t)
	for _, mode := range []string{"good", "fail", "none", "no-midi", "midi", "loud", "slow"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "-score with spaces:part.ly")
			writeScore(t, source, mode)
			userPDF := filepath.Join(dir, "-score with spaces:part.pdf")
			writeScore(t, userPDF, "user PDF")
			timeout := 3 * time.Second
			if mode == "slow" {
				timeout = 150 * time.Millisecond
			}
			r := compile(context.Background(), Options{Source: source, Executable: executable, Timeout: timeout, IncludeDirs: []string{dir}}, t.TempDir(), 7)
			if r.Generation != 7 {
				t.Fatal(r)
			}
			switch mode {
			case "fail":
				if r.Err == nil || len(r.Diagnostics) != 1 || r.Diagnostics[0].File != source || r.Diagnostics[0].Line != 12 || r.Diagnostics[0].Column != 3 || !strings.Contains(r.Diagnostics[0].Message, "continued detail") {
					t.Fatalf("bad diagnostics: %+v", r)
				}
			case "none":
				if r.Err == nil || !strings.Contains(r.Err.Error(), "no PDF or MIDI") {
					t.Fatal(r.Err)
				}
			case "slow":
				if r.Err == nil || !strings.Contains(r.Err.Error(), "timed out") || r.Duration > 2*time.Second || len(r.PDFs) != 0 {
					t.Fatalf("timeout published partial output or failed cleanup: %+v", r)
				}
			default:
				if r.Err != nil {
					t.Fatal(r.Err)
				}
				if (mode != "midi" && len(r.PDFs) != 1) || (mode != "no-midi" && len(r.MIDIs) != 1) {
					t.Fatalf("bad artifacts: %+v", r)
				}
				if mode == "loud" && (!r.Truncated || len(r.Log) != 256*1024) {
					t.Fatal("unbounded or missing log")
				}
			}
			if data, _ := os.ReadFile(userPDF); string(data) != "user PDF" {
				t.Fatal("overwrote existing PDF")
			}
			cwd, _ := os.ReadFile(source + ".cwd")
			actual, _ := filepath.EvalSymlinks(strings.TrimSpace(string(cwd)))
			want, _ := filepath.EvalSymlinks(dir)
			if actual != want {
				t.Fatalf("compiler working directory changed: %q", cwd)
			}
			if data, _ := os.ReadFile(source + ".args"); !strings.HasSuffix(string(data), source+"\n") || strings.Contains(string(data), "\n--\n") || !strings.Contains(string(data), "--include="+dir+"\n") {
				t.Fatalf("unsafe arguments: %q", data)
			}
		})
	}
}

func TestMissingCompilerAndScore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "score.ly")
	writeScore(t, file, "good")
	r := compile(context.Background(), Options{Source: file, Executable: filepath.Join(t.TempDir(), "missing")}, t.TempDir(), 1)
	if r.Err == nil || !strings.Contains(r.Err.Error(), "LilyPond not found") {
		t.Fatal(r.Err)
	}
	r = compile(context.Background(), Options{Source: file + ".missing"}, t.TempDir(), 2)
	if r.Err == nil || !strings.Contains(r.Err.Error(), "save it in Vim first") {
		t.Fatal(r.Err)
	}
}

func TestRealLilyPond(t *testing.T) {
	tool := os.Getenv("LYRA_TEST_LILYPOND")
	if tool == "" {
		t.Skip("set LYRA_TEST_LILYPOND for real compiler qualification")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	source := filepath.Join(dir, "-real score:with spaces.ly")
	score, err := os.ReadFile("../testdata/scale.ly")
	if err != nil {
		t.Fatal(err)
	}
	writeScore(t, source, string(score))
	r := compile(context.Background(), Options{Source: source, Executable: tool}, t.TempDir(), 1)
	if r.Err != nil || len(r.PDFs) != 1 || len(r.MIDIs) != 1 {
		t.Fatalf("real compiler: %v\n%s", r.Err, r.Log)
	}
	t.Logf("LilyPond produced PDF and MIDI in %s", r.Duration)
	if err := os.Mkdir(filepath.Join(dir, "parts"), 0700); err != nil {
		t.Fatal(err)
	}
	writeScore(t, filepath.Join(dir, "parts", "notes.ily"), `\include "rhythm.ily"`)
	writeScore(t, filepath.Join(dir, "parts", "rhythm.ily"), `notes = { c'4 d' e' f' }`)
	writeScore(t, source, "\\version \"2.24.3\"\n\\include \"parts/notes.ily\"\n\\score { \\notes \\layout {} \\midi {} }\n")
	r = compile(context.Background(), Options{Source: source, Executable: tool}, t.TempDir(), 2)
	if r.Err != nil || len(r.PDFs) != 1 || len(r.MIDIs) != 1 {
		t.Fatalf("real nested includes: %v\n%s", r.Err, r.Log)
	}
	writeScore(t, source, "\\version \"2.24.3\"\n{ invalidNOTE }\n")
	r = compile(context.Background(), Options{Source: source, Executable: tool}, t.TempDir(), 3)
	if r.Err == nil || len(r.Diagnostics) == 0 {
		t.Fatalf("real error was not parsed: %v\n%s", r.Err, r.Log)
	}
}
