//go:build darwin || linux

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestAdoptionWatcherAndValidation(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "source.json")
	w := watchAdoption(context.Background(), marker)
	defer w.Close()
	destination := filepath.Join(dir, "rain 雨 with spaces.ly")
	data, _ := json.Marshal(destination)
	if err := os.WriteFile(marker+".tmp", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(marker+".tmp", marker); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-w.events:
		if event.err != nil || event.path != destination {
			t.Fatal(event)
		}
	case <-time.After(time.Second):
		t.Fatal("atomic first-save notification was not observed")
	}
	for _, invalid := range []string{`"relative.ly"`, `{}`, `""`, `"/score.ly" junk`, strings.Repeat("x", (64<<10)+1)} {
		if err := os.WriteFile(marker, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readAdoption(marker); err == nil {
			t.Fatal("invalid first-save notification was accepted")
		}
	}
	stopped := watchAdoption(context.Background(), filepath.Join(dir, "absent"))
	stopped.Close()
	if _, ok := <-stopped.events; ok {
		t.Fatal("closed watcher published an adoption")
	}
	retryMarker := filepath.Join(dir, "retry")
	if err := os.Mkdir(retryMarker, 0700); err != nil {
		t.Fatal(err)
	}
	retry := watchAdoption(context.Background(), retryMarker)
	defer retry.Close()
	select {
	case event := <-retry.events:
		if event.err == nil {
			t.Fatal("notification failure was hidden")
		}
	case <-time.After(time.Second):
		t.Fatal("notification failure was not reported")
	}
	if err := os.Remove(retryMarker); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(retryMarker, data, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-retry.events:
		if event.err != nil || event.path != destination {
			t.Fatal("notification retry failed", event)
		}
	case <-time.After(time.Second):
		t.Fatal("repaired notification was not adopted")
	}
}

// Exercise the no-filename flow through the complete app, not only Vim's hook.
// The deterministic compiler records its actual source and working directory.
func TestUntitledFullTUI(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("requires traditional Vim")
	}
	for _, interrupted := range []bool{false, true} {
		name := "first save"
		if interrupted {
			name = "interrupted sketch"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "ready")
			rc := "set nocompatible noswapfile viminfo= nomore\nset shortmess+=I\n" +
				"autocmd VimEnter * call writefile([expand('%:p')], '" + ready + "')\n"
			if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
				t.Fatal(err)
			}
			compiler := filepath.Join(dir, "compiler")
			script := `#!/bin/sh
for arg do
  case "$arg" in --output=*) output=${arg#*=};; esac
  source=$arg
done
printf '%%PDF-1.4 untitled-test' > "$output.pdf"
printf 'run\n' >> "$source.builds"
pwd > "$source.cwd"
`
			if err := os.WriteFile(compiler, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestAppProcess$")
			for _, item := range os.Environ() {
				key, _, _ := strings.Cut(item, "=")
				if key == "HOME" || key == "VIMINIT" || key == "EXINIT" || key == "GVIMINIT" || strings.HasPrefix(key, "LYRA_TEST_") {
					continue
				}
				cmd.Env = append(cmd.Env, item)
			}
			cmd.Env = append(cmd.Env, "HOME="+dir, "TERM=xterm-256color", "LYRA_TEST_PROCESS=1", "LYRA_TEST_FILE=", "LYRA_TEST_COMPILER="+compiler)
			if interrupted {
				cmd.Env = append(cmd.Env, "LYRA_TEST_SIGNAL=1")
			}
			master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 32})
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var output bytes.Buffer
			readDone, processDone := make(chan struct{}), make(chan struct{})
			var processErr error
			go func() { processErr = cmd.Wait(); close(processDone) }()
			go func() {
				defer close(readDone)
				chunk := make([]byte, 8192)
				for {
					n, err := master.Read(chunk)
					mu.Lock()
					output.Write(chunk[:n])
					mu.Unlock()
					if err != nil {
						return
					}
				}
			}()
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				<-processDone
				_ = master.Close()
				<-readDone
				if t.Failed() {
					mu.Lock()
					defer mu.Unlock()
					t.Logf("untitled TUI output: %q", output.String())
				}
			})
			write := func(data string) {
				t.Helper()
				if _, err := io.WriteString(master, data); err != nil {
					t.Fatal(err)
				}
			}
			hasText := func(want string) bool {
				mu.Lock()
				data := bytes.Clone(output.Bytes())
				mu.Unlock()
				return strings.Contains(strings.Join(hostScreen(t, data, 120, 32), "\n"), want)
			}
			waitFor(t, "new sketch frame", func() bool { return hasText("New sketch") })
			waitFor(t, "Vim reports the temporary filename", func() bool { _, err := os.Stat(ready); return err == nil })
			pathBytes, err := os.ReadFile(ready)
			if err != nil {
				t.Fatal(err)
			}
			sketch := strings.TrimSpace(string(pathBytes))
			if !strings.HasPrefix(filepath.Base(filepath.Dir(sketch)), "lyra-sketch-") || filepath.Ext(sketch) != ".ly" {
				t.Fatal("no private musical sketch was opened", sketch)
			}
			// This directory is owned by this test's newly launched session.
			t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(sketch)) })
			if _, err := os.Stat(sketch + ".builds"); !os.IsNotExist(err) {
				t.Fatal("temporary sketch compiled before being named")
			}
			contents := "\\include \"notes.ily\"\n{ c'4 d' e' f' }\n"
			write("i\x1b[200~" + strings.TrimSuffix(contents, "\n") + "\x1b[201~\x1b")
			if interrupted {
				write(":call writefile([string(&modified)], '" + filepath.Join(dir, "modified") + "')\r")
				waitFor(t, "unsaved sketch modification", func() bool { b, _ := os.ReadFile(filepath.Join(dir, "modified")); return string(b) == "1\n" })
				_ = cmd.Process.Signal(syscall.SIGTERM)
			} else {
				write(":wq\r")
				waitFor(t, "embedded filename prompt", func() bool { return hasText("Save score as:") })
				write("\x1b")
				waitFor(t, "cancel keeps Vim open", func() bool { return hasText("save cancelled") })
				write("\r:w\r")
				waitFor(t, "retry filename prompt", func() bool { return hasText("Save score as:") })
				target := filepath.Join(dir, "my nocturne 雨.ly")
				include := filepath.Join(dir, "notes.ily")
				if err := os.WriteFile(include, []byte("% original include\n"), 0600); err != nil {
					t.Fatal(err)
				}
				write("\x15" + target + "\r")
				waitFor(t, "first save uses chosen path", func() bool { b, _ := os.ReadFile(target); return string(b) == contents })
				waitFor(t, "first save retargets compiler", func() bool { b, _ := os.ReadFile(target + ".builds"); return len(b) > 0 })
				waitFor(t, "header adopts the chosen name", func() bool { return hasText(filepath.Base(target)) && !hasText(" · sketch") })
				cwd, _ := os.ReadFile(target + ".cwd")
				if !sameFile(strings.TrimSpace(string(cwd)), dir) {
					t.Fatal("compiler did not use the saved score's directory", string(cwd))
				}
				before, _ := os.ReadFile(target + ".builds")
				write(":call append(line('$'), '% second save')\r:w\r")
				waitFor(t, "later saves compile the adopted score", func() bool { b, _ := os.ReadFile(target + ".builds"); return len(b) > len(before) })
				before, _ = os.ReadFile(target + ".builds")
				if err := os.WriteFile(include, []byte("% changed include\n"), 0600); err != nil {
					t.Fatal(err)
				}
				waitFor(t, "include watch follows saved score", func() bool { b, _ := os.ReadFile(target + ".builds"); return len(b) > len(before) })
				write(":q\r")
			}
			select {
			case <-processDone:
				if processErr != nil {
					t.Fatal(processErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("untitled TUI did not exit")
			}
			_, err = os.Stat(sketch)
			if interrupted && err != nil {
				t.Fatal("interrupted sketch was discarded", err)
			}
			if !interrupted && !os.IsNotExist(err) {
				t.Fatal("saved sketch was not cleaned up", err)
			}
		})
	}
}
