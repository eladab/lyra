//go:build darwin || linux

package app

import (
	"bytes"
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
	"lyra/internal/scoretemplate"
)

func TestStartupChooserFullTUI(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("requires traditional Vim")
	}
	for _, action := range []string{"open", "create", "template", "cancel", "signal", "template-cancel", "template-signal"} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "vim-started")
			rc := "set nocompatible noswapfile viminfo= nomore\nset shortmess+=I\n" +
				"autocmd VimEnter * call writefile([expand('%:p'), string(&columns)], '" + ready + "')\n" +
				"autocmd VimEnter * call writefile(getline(1, '$'), '" + filepath.Join(dir, "opened-content") + "')\n"
			if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
				t.Fatal(err)
			}
			existing := filepath.Join(dir, "existing 雨.ly")
			if err := os.WriteFile(existing, []byte("{ c4 }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(dir, "new scores")
			if err := os.Mkdir(folder, 0700); err != nil {
				t.Fatal(err)
			}
			compiler := filepath.Join(dir, "compiler")
			script := `#!/bin/sh
for arg do
  case "$arg" in --output=*) output=${arg#*=};; esac
  source=$arg
done
printf '%%PDF-1.4 launch-test' > "$output.pdf"
pwd > "$source.cwd"
printf 'run\n' >> "$source.builds"
`
			if err := os.WriteFile(compiler, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestAppProcess$")
			cmd.Dir = dir
			for _, item := range os.Environ() {
				key, _, _ := strings.Cut(item, "=")
				if key == "HOME" || key == "VIMINIT" || key == "EXINIT" || key == "GVIMINIT" || strings.HasPrefix(key, "LYRA_TEST_") {
					continue
				}
				cmd.Env = append(cmd.Env, item)
			}
			cmd.Env = append(cmd.Env, "HOME="+dir, "TERM=xterm-256color", "LYRA_TEST_PROCESS=1", "LYRA_TEST_FILE=", "LYRA_TEST_COMPILER="+compiler)
			if action == "signal" || action == "template-signal" {
				cmd.Env = append(cmd.Env, "LYRA_TEST_SIGNAL=1")
			}
			cols, rows := 120, 32
			if action == "template" {
				cols, rows = 64, 20
			}
			master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
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
					data := output.Bytes()
					t.Logf("startup output: %q", data[max(0, len(data)-12_000):])
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
				return strings.Contains(strings.Join(hostScreen(t, data, cols, rows), "\n"), want)
			}
			waitFor(t, "startup menu", func() bool { return hasText("Begin a composition") && hasText("Create a score") })
			if _, err := os.Stat(ready); !os.IsNotExist(err) {
				t.Fatal("Vim started before a score was chosen")
			}
			target := existing
			switch action {
			case "cancel":
				write("q")
			case "signal":
				write("n")
				waitFor(t, "new-score folder browser", func() bool { return hasText("Choose a folder") })
				write("n")
				waitFor(t, "new-score form", func() bool { return hasText("File location") })
				_ = cmd.Process.Signal(syscall.SIGTERM)
			case "open":
				cols, rows = 64, 20
				if err := pty.Setsize(master, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
					t.Fatal(err)
				}
				_ = cmd.Process.Signal(syscall.SIGWINCH)
				waitFor(t, "resized chooser", func() bool { return hasText("Begin a composition") })
				write("o")
				waitFor(t, "open browser", func() bool { return hasText("existing 雨.ly") })
				write("/")
				waitFor(t, "open-path field", func() bool { return hasText("File location") })
				write("\x15\x1b[200~" + target + "\x1b[201~\r")
			case "create", "template", "template-cancel", "template-signal":
				write("n")
				waitFor(t, "create browser", func() bool { return hasText("Choose a folder") })
				write("n")
				waitFor(t, "create-path field", func() bool { return hasText("File location") })
				write("\x15\x1b[200~" + existing + "\x1b[201~\r")
				waitFor(t, "existing file stays protected", func() bool { return hasText("already exists") })
				if data, _ := os.ReadFile(existing); string(data) != "{ c4 }\n" {
					t.Fatal("create replaced existing source")
				}
				target = filepath.Join(folder, "quiet rain 雨.ly")
				write("\x15\x1b[200~" + strings.TrimSuffix(target, ".ly") + "\x1b[201~\r")
				waitFor(t, "template selection follows the filename", func() bool { return hasText("Choose a template or start blank") })
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatal("startup created a file before template confirmation")
				}
				if _, err := os.Stat(ready); !os.IsNotExist(err) {
					t.Fatal("Vim started before template confirmation")
				}
				switch action {
				case "template-cancel":
					write("\x03")
				case "template-signal":
					_ = cmd.Process.Signal(syscall.SIGTERM)
				case "template":
					write("\x1b[200~quartet string\x1b[201~")
					waitFor(t, "fuzzy template selection", func() bool { return hasText("String quartet · String quartet") })
					write("\r")
				default:
					write("\r") // Default choice is a genuinely empty score.
				}
			}
			if action == "open" || action == "create" || action == "template" {
				var lines []string
				waitFor(t, "chosen score starts Vim", func() bool {
					started, err := os.ReadFile(ready)
					lines = strings.Split(strings.TrimSpace(string(started)), "\n")
					return err == nil && len(lines) == 2 && lines[0] != "" && lines[1] != ""
				})
				left, _ := os.Stat(lines[0])
				right, _ := os.Stat(target)
				if left == nil || right == nil || !os.SameFile(left, right) {
					t.Fatal("Vim opened a different file", lines)
				}
				if (action == "open" || action == "template") && lines[1] != "64" {
					t.Fatal("editor did not use the resized terminal", lines)
				}
				if action == "template" {
					want := scoretemplate.Search("quartet string")[0].Code
					waitFor(t, "Vim opens the complete selected template", func() bool {
						data, _ := os.ReadFile(filepath.Join(dir, "opened-content"))
						return string(data) == want
					})
					data, _ := os.ReadFile(target)
					if string(data) != want {
						t.Fatal("template contents were not initialized on disk")
					}
				} else if action == "create" {
					data, _ := os.ReadFile(target)
					if len(data) != 0 {
						t.Fatal("blank choice added unexpected contents")
					}
				}
				waitFor(t, "selected score builds", func() bool { b, _ := os.ReadFile(target + ".builds"); return len(b) > 0 })
				cwd, _ := os.ReadFile(target + ".cwd")
				compiledDir, _ := os.Stat(strings.TrimSpace(string(cwd)))
				sourceDir, _ := os.Stat(filepath.Dir(target))
				if compiledDir == nil || sourceDir == nil || !os.SameFile(compiledDir, sourceDir) {
					t.Fatal("compiler did not follow the selected score's directory", string(cwd))
				}
				before, _ := os.ReadFile(target + ".builds")
				write("ggdGi{ c'4 d'4 }\x1b:w\r")
				waitFor(t, "first save is an ordinary Vim save", func() bool { b, _ := os.ReadFile(target); return string(b) == "{ c'4 d'4 }\n" })
				waitFor(t, "save rebuilds the chosen score", func() bool { b, _ := os.ReadFile(target + ".builds"); return len(b) > len(before) })
				if hasText("Save score as:") {
					t.Fatal("startup selection still triggered a first-save prompt")
				}
				write(":q\r")
			}
			select {
			case <-processDone:
				if processErr != nil {
					t.Fatal(processErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("startup or reader handoff did not exit cleanly")
			}
			if action == "cancel" || action == "signal" || action == "template-cancel" || action == "template-signal" {
				if _, err := os.Stat(ready); !os.IsNotExist(err) {
					t.Fatal("cancelled chooser started Vim")
				}
				files, err := os.ReadDir(folder)
				if err != nil || len(files) != 0 {
					t.Fatal("cancelled chooser created a score", err)
				}
			} else if _, err := os.Stat(target); err != nil {
				t.Fatal("normal exit removed the chosen file", err)
			}
		})
	}
}
