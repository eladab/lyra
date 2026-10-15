//go:build darwin || linux

package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const paletteScore = `\version "2.26.0"
\score {
  \relative c' {
    cis'4. r8 <ees g b>2 \mf
    \tuplet 3/2 { c8 d e } % c4 \forte
    \markup { a bright \bold "c4" }
    #(define (color-note pitch) (ly:make-pitch 0 0 0))
    %{ c4
       \relative d8 %} f4
  }
}
`

func paletteVim(t *testing.T, disabled, existingSyntax bool) (*Session, string) {
	t.Helper()
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("traditional Vim is required for PTY integration tests")
	}
	dir := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(".vimrc", "set nocompatible\nset noswapfile viminfo= nomore\nset shortmess+=I\n"+
		"syntax on\nfiletype on\n"+
		"highlight Normal guifg=#dadada ctermfg=252\n"+
		"highlight String guifg=#123456 ctermfg=77\n"+
		"let g:lyra_test_config = 'loaded'\n"+
		"nnoremap <F5> iUserMapping<Esc>\n"+
		"autocmd VimEnter * call writefile(['ready'], '"+filepath.Join(dir, "ready")+"')\n")
	write(".vim/plugin/user.vim", "let g:lyra_user_plugin_loaded = 1\n")
	if existingSyntax {
		write(".vim/ftdetect/lilypond.vim", "autocmd BufRead,BufNewFile *.ly,*.ily setfiletype lilypond\n")
		write(".vim/syntax/lilypond.vim", "syntax region UserLilyMatcher start=/{/ end=/}/ contains=UserLilyNote\n"+
			"syntax match UserLilyNote /[a-g][0-9]*/ contained\n"+
			"highlight UserLilyNote guifg=#ff0000 ctermfg=196\n"+
			"let b:current_syntax = 'lilypond'\n")
	}
	write("score.ly", paletteScore)
	write("included.ily", "c'4 r8 \\mf\n")
	write("other.txt", "c4 \\relative\n")
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "HOME" && key != "VIMINIT" && key != "EXINIT" && key != "GVIMINIT" {
			env = append(env, item)
		}
	}
	env = append(env, "HOME="+dir)
	s, err := Start(Options{File: filepath.Join(dir, "score.ly"), Cols: 100, Rows: 24, Env: env, DisableLilyPondSyntax: disabled})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("screen=%q", s.Terminal.Snapshot().Content)
		}
	})
	eventually(t, "Vim normal startup", func() bool { return readEquals(filepath.Join(dir, "ready"), "ready\n") })
	return s, dir
}

func vimValues(t *testing.T, s *Session, dir, expression string) []string {
	t.Helper()
	path := filepath.Join(dir, "result-"+strconv.FormatInt(int64(len(t.Name())), 10))
	_ = os.Remove(path)
	send(t, s, fmt.Sprintf(":call writefile(%s, '%s')\r", expression, path))
	eventually(t, "Vim inspection result", func() bool { _, err := os.Stat(path); return err == nil })
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestLilyPondPaletteWithUserVimStartup(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-syntax=%t", existing), func(t *testing.T) {
			s, dir := paletteVim(t, false, existing)
			lines := strings.Split(paletteScore, "\n")
			checks := []struct {
				line         int
				token, group string
			}{
				{1, `\version`, "LyraLilyCommand"},
				{1, `"2.26.0"`, "LyraLilyString"},
				{2, `\score`, "LyraLilyStructure"},
				{4, "cis", "LyraLilyNote"},
				{4, "'4", "LyraLilyOctave"},
				{4, "4.", "LyraLilyDuration"},
				{4, "r8", "LyraLilyRest"},
				{4, "ees", "LyraLilyNote"},
				{4, `\mf`, "LyraLilyDynamic"},
				{5, `\tuplet`, "LyraLilyStructure"},
				{5, "% c4", "LyraLilyComment"},
				{6, "a bright", "LyraLilyMarkup"},
				{6, `\bold`, "LyraLilyCommand"},
				{6, `"c4"`, "LyraLilyString"},
				{7, "define", "LyraLilySchemeFunction"},
				{7, "ly:make-pitch", "LyraLilySchemeFunction"},
				{8, "c4", "LyraLilyBlockComment"},
				{9, `\relative`, "LyraLilyBlockComment"},
				{9, "f4", "LyraLilyNote"},
			}
			expressions := []string{}
			for _, check := range checks {
				col := strings.Index(lines[check.line-1], check.token) + 1
				if col < 1 {
					t.Fatalf("missing token %q", check.token)
				}
				expressions = append(expressions, fmt.Sprintf("synIDattr(synID(%d, %d, 1), 'name')", check.line, col))
			}
			values := vimValues(t, s, dir, "["+strings.Join(expressions, ",")+"]")
			for i, check := range checks {
				if values[i] != check.group {
					t.Errorf("line %d token %q: got %q, want %q", check.line, check.token, values[i], check.group)
				}
			}
			values = vimValues(t, s, dir, "[g:lyra_test_config,string(g:lyra_user_plugin_loaded),maparg('<F5>', 'n'),synIDattr(hlID('Normal'),'fg','gui'),synIDattr(hlID('String'),'fg','gui'),synIDattr(hlID('LyraLilyNote'),'fg','gui'),synIDattr(hlID('LyraLilyCommand'),'fg','cterm'),synIDattr(hlID('LyraLilyDuration'),'fg','cterm')]")
			want := []string{"loaded", "1", "iUserMapping<Esc>", "#dadada", "#123456", "#82c9c1", "146", "180"}
			if strings.Join(values, "\n") != strings.Join(want, "\n") {
				t.Errorf("startup/colors/mapping changed: got %q want %q", values, want)
			}
			send(t, s, ":edit "+filepath.Join(dir, "included.ily")+"\r")
			values = vimValues(t, s, dir, "[synIDattr(synID(1,1,1),'name')]")
			if values[0] != "LyraLilyNote" {
				t.Errorf("include highlighting: %q", values)
			}
			send(t, s, ":edit "+filepath.Join(dir, "other.txt")+"\r")
			values = vimValues(t, s, dir, "[synIDattr(synID(1,1,1),'name'),synIDattr(hlID('String'),'fg','gui'),string(exists('b:lyra_lilypond_syntax'))]")
			if strings.Join(values, "\n") != "\n#123456\n0" {
				t.Errorf("non-LilyPond buffer affected: %q", values)
			}
			syntaxDir := s.syntaxDir
			send(t, s, ":q\r")
			select {
			case <-s.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("Vim did not exit")
			}
			if _, err := os.Stat(syntaxDir); !os.IsNotExist(err) {
				t.Errorf("session syntax directory remains: %v", err)
			}
		})
	}
}

func TestLilyPondPaletteOptOut(t *testing.T) {
	s, dir := paletteVim(t, true, true)
	values := vimValues(t, s, dir, "[synIDattr(synID(4,5,1),'name'),synIDattr(hlID('UserLilyNote'),'fg','gui'),string(exists('b:lyra_lilypond_syntax')),g:lyra_test_config]")
	if strings.Join(values, "\n") != "UserLilyNote\n#ff0000\n0\nloaded" {
		t.Fatalf("opt-out did not preserve user syntax: %q", values)
	}
	if s.syntaxDir != "" {
		t.Fatal("opt-out created a syntax directory")
	}
}
