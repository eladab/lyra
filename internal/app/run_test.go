//go:build darwin || linux

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// TestAppProcess becomes a standalone Lyra process only inside our test PTY.
func TestAppProcess(t *testing.T) {
	if os.Getenv("LYRA_TEST_PROCESS") != "1" {
		return
	}
	before, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), termiosGetRequest)
	if err != nil {
		t.Fatal(err)
	}
	signalContext, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, 15*time.Second)
	defer cancel()
	err = Run(ctx, Options{File: os.Getenv("LYRA_TEST_FILE"), LilyPond: os.Getenv("LYRA_TEST_COMPILER"), SoundFont: os.Getenv("LYRA_TEST_SOUNDFONT"), AudioOutput: "null"}, os.Stdin, os.Stdout)
	after, stateErr := unix.IoctlGetTermios(int(os.Stdin.Fd()), termiosGetRequest)
	// Darwin may set PENDIN after tcsetattr even when all user-controlled
	// settings are restored. It describes pending kernel input reprocessing.
	before.Lflag &^= unix.PENDIN
	if after != nil {
		after.Lflag &^= unix.PENDIN
	}
	if stateErr != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("terminal settings were not restored: %v before=%#v after=%#v", stateErr, before, after)
	}
	if err != nil && os.Getenv("LYRA_TEST_SIGNAL") != "1" {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, name string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout: " + name)
}

// Decode the actual host output with LF retaining the current column, as it
// does in a terminal without kernel ONLCR processing. Checking View strings
// alone misses mismatches between raw mode and the renderer's cursor moves.
func hostScreen(t *testing.T, output []byte, cols, rows int) []string {
	t.Helper()
	screen := vt.NewEmulator(cols, rows)
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		_, _ = io.Copy(io.Discard, screen)
	}()
	defer func() {
		_ = screen.InputPipe().(io.Closer).Close()
		<-responsesDone
		_ = screen.Close()
	}()
	if _, err := screen.Write(output); err != nil {
		t.Fatal(err)
	}
	return strings.Split(ansi.Strip(screen.Render()), "\n")
}

func checkSplitScreen(t *testing.T, output []byte, cols, rows int) {
	t.Helper()
	lines := hostScreen(t, output, cols, rows)
	l := measure(cols, rows)
	if !strings.HasPrefix(lines[0], " LYRA") || !strings.HasPrefix(lines[1], " ● VIM") {
		t.Fatalf("host headings drifted: %q", lines[:2])
	}
	for y := 2; y < rows-1; y++ {
		cells := []rune(lines[y])
		if len(cells) <= l.editorCols || cells[l.editorCols] != '│' {
			t.Fatalf("host divider drifted on row %d: %q", y, lines[y])
		}
	}
	for y := 3; y < 10; y++ {
		if !strings.HasPrefix(lines[y], "~") {
			t.Fatalf("empty Vim row %d drifted: %q", y, lines[y])
		}
	}
	for y, want := range map[int]string{4: "Ctrl+\\ d: build diagnostics"} {
		if !strings.HasPrefix(string([]rune(lines[y])[l.editorCols+1:]), want) {
			t.Fatalf("host preview row %d obscured: %q", y, lines[y])
		}
	}
	if !strings.HasPrefix(lines[rows-1], " Ctrl+\\:") {
		t.Fatalf("host footer drifted: %q", lines[rows-1])
	}
}

func TestFullTUI(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("requires traditional Vim")
	}
	previewFixture := ""
	midiFixture := ""
	if exe := os.Getenv("LYRA_TEST_LILYPOND"); exe != "" {
		for _, tool := range []string{"pdfinfo", "pdftoppm"} {
			if _, err := exec.LookPath(tool); err != nil {
				t.Fatal("real preview test requires Poppler: " + tool)
			}
		}
		dir := t.TempDir()
		source := filepath.Join(dir, "two pages.ly")
		contents := "\\version \"2.24.3\"\n\\book { \\bookpart { \\score { { c'4 d' e' f' } \\layout {} } } \\bookpart { \\score { { g'4 a' b' c'' } \\layout {} } } }\n"
		if err := os.WriteFile(source, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "--pdf", "--output="+filepath.Join(dir, "score"), source)
		cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("real multi-page engraving: %v %s", err, out)
		}
		previewFixture = filepath.Join(dir, "score.pdf")
		if os.Getenv("LYRA_TEST_SOUNDFONT") != "" {
			fixture, err := filepath.Abs("../testdata/scale.ly")
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, exe, "--output="+filepath.Join(dir, "audio"), fixture)
			cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("real audio fixture: %v %s", err, out)
			}
			midiFixture = filepath.Join(dir, "audio.midi")
		}
	}
	for _, signalExit := range []bool{false, true} {
		t.Run(fmt.Sprint("signal=", signalExit), func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "composition.ly")
			compiler := filepath.Join(dir, "lilypond")
			compilerScript := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in --output=*) output=${arg#--output=};; esac
  source=$arg
done
if [ -n "$LYRA_TEST_PDF" ]; then cp "$LYRA_TEST_PDF" "$output.pdf"; else printf '%%PDF-1.4 app-test' > "$output.pdf"; fi
if [ -n "$LYRA_TEST_MIDI" ]; then cp "$LYRA_TEST_MIDI" "$output.midi"; fi
echo run >> "$source.builds"
`
			if err := os.WriteFile(compiler, []byte(compilerScript), 0700); err != nil {
				t.Fatal(err)
			}
			ready := filepath.Join(dir, "ready")
			rc := "set nocompatible noswapfile viminfo= nomore\nset shortmess+=I\nset ttimeoutlen=20\n" +
				"set termguicolors laststatus=2\n" +
				"highlight Normal guifg=#dddddd guibg=#262626\n" +
				"highlight EndOfBuffer guifg=#555555 guibg=#262626\n" +
				"autocmd VimEnter * call writefile(['ready'], '" + ready + "')\n"
			if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestAppProcess$")
			for _, item := range os.Environ() {
				key, _, _ := strings.Cut(item, "=")
				switch key {
				case "HOME", "VIMINIT", "EXINIT", "GVIMINIT", "TERM", "TERM_PROGRAM", "COLORTERM", "NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "TMUX", "STY":
				default:
					cmd.Env = append(cmd.Env, item)
				}
			}
			cmd.Env = append(cmd.Env, "HOME="+dir, "TERM=xterm-256color", "TERM_PROGRAM=ghostty", "COLORTERM=truecolor", "LYRA_TEST_PROCESS=1", "LYRA_TEST_FILE="+file, "LYRA_TEST_COMPILER="+compiler)
			cmd.Env = append(cmd.Env, "LYRA_TEST_PDF="+previewFixture)
			cmd.Env = append(cmd.Env, "LYRA_TEST_MIDI="+midiFixture)
			if signalExit {
				cmd.Env = append(cmd.Env, "LYRA_TEST_SIGNAL=1")
			}
			master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 32})
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			defer cmd.Process.Kill()
			var mu sync.Mutex
			var output bytes.Buffer
			readerDone := make(chan struct{})
			go func() {
				defer close(readerDone)
				b := make([]byte, 8192)
				replied := false
				probe := regexp.MustCompile("\x1b_Ga=q,i=([0-9]+),")
				for {
					n, err := master.Read(b)
					mu.Lock()
					output.Write(b[:n])
					var response string
					if !replied {
						if match := probe.FindSubmatch(output.Bytes()); len(match) == 2 {
							response = "\x1b_Gi=" + string(match[1]) + ";OK\x1b\\\x1b[6;20;10t"
							replied = true
						}
					}
					mu.Unlock()
					if response != "" {
						_, _ = io.WriteString(master, response)
					}
					if err != nil {
						return
					}
				}
			}()
			t.Cleanup(func() {
				if t.Failed() {
					mu.Lock()
					defer mu.Unlock()
					t.Logf("TUI output: %q", output.String())
				}
			})
			waitFor(t, "Vim startup", func() bool { _, err := os.Stat(ready); return err == nil })
			waitFor(t, "empty-buffer Bubble Tea frame", func() bool {
				mu.Lock()
				defer mu.Unlock()
				return strings.Count(output.String(), "~") >= 10 && strings.Contains(output.String(), "quit")
			})
			mu.Lock()
			initialOutput := bytes.Clone(output.Bytes())
			mu.Unlock()
			checkSplitScreen(t, initialOutput, 120, 32)
			write := func(s string) {
				t.Helper()
				if _, err := io.WriteString(master, s); err != nil {
					t.Fatal(err)
				}
			}
			write("ihello é\x1b:w\r")
			waitFor(t, "TUI forwards edit/save", func() bool { b, _ := os.ReadFile(file); return string(b) == "hello é\n" })
			waitFor(t, "saved score compiles in the background", func() bool { b, _ := os.ReadFile(file + ".builds"); return len(b) > 0 })
			if midiFixture != "" {
				hasOutput := func(want string) bool { mu.Lock(); defer mu.Unlock(); return strings.Contains(output.String(), want) }
				write("\x1ca\x1c ")
				waitFor(t, "full app starts actual MIDI playback", func() bool { return hasOutput("Playing") })
				write("\x1c ")
				waitFor(t, "full app pauses playback", func() bool { return hasOutput("Paused") })
				write("\x1cs")
				waitFor(t, "full app stops playback", func() bool { return hasOutput("Stopped") })
				write("\x1ce\x1cp\x1ce")
			}
			if previewFixture != "" {
				imageCount := func() int { mu.Lock(); defer mu.Unlock(); return strings.Count(output.String(), "\x1b_Ga=T") }
				waitFor(t, "real PDF image emitted through Bubble Tea", func() bool { return imageCount() >= 1 })
				mu.Lock()
				hostOutput := output.String()
				mu.Unlock()
				images := decodeHostImages(t, hostOutput)
				if len(images) == 0 || images[0].Dx() != 600 || images[0].Dy() != 560 {
					t.Fatalf("PDF viewport not cropped to preview cells: %v", images)
				}
				before := imageCount()
				write("\x1cpn")
				waitFor(t, "next PDF page", func() bool { return imageCount() > before })
				mu.Lock()
				current := bytes.Clone(output.Bytes())
				mu.Unlock()
				if !strings.Contains(strings.Join(hostScreen(t, current, 120, 32), "\n"), "Page 2/2") {
					t.Fatal("page navigation did not reach second engraved page")
				}
				before = imageCount()
				write("\x1ct")
				waitFor(t, "night theme redraws the actual PDF", func() bool { return imageCount() > before })
				mu.Lock()
				current = bytes.Clone(output.Bytes())
				mu.Unlock()
				if !strings.Contains(strings.Join(hostScreen(t, current, 120, 32), "\n"), "Night") {
					t.Fatal("night theme did not reach the displayed preview status")
				}
				before = imageCount()
				write("tt")
				waitFor(t, "direct theme keys return to warm paper", func() bool { return imageCount() > before })
				before = imageCount()
				write("+jjr")
				waitFor(t, "zoom, scroll and reload PDF", func() bool { return imageCount() > before })
				write("\x1cd")
				waitFor(t, "diagnostics clear image", func() bool {
					mu.Lock()
					defer mu.Unlock()
					lastImage := strings.LastIndex(output.String(), "\x1b_Ga=T")
					lastDelete := strings.LastIndex(output.String(), "\x1b_Ga=d")
					return lastDelete > lastImage
				})
				write("\x1cp\x1ce")
			}
			beforeBuild, _ := os.ReadFile(file + ".builds")
			write("\x1cb")
			waitFor(t, "manual prefix rebuild", func() bool { b, _ := os.ReadFile(file + ".builds"); return len(b) > len(beforeBuild) })
			// Bytes typed in the preview must never mutate the Vim buffer.
			write("\x1cpithis must stay out of Vim\x1ce:w\r")
			// Prefix resize reaches the actual child before terminal resize.
			write("\x1c]")
			splitMarker := filepath.Join(dir, "split-dimensions")
			write(":call writefile([string(&columns)], '" + splitMarker + "')\r")
			waitFor(t, "adjustable split reaches Vim", func() bool { b, _ := os.ReadFile(splitMarker); return string(b) == "65\n" })
			write("\x1c=")
			// A command marker proves all preceding input was consumed in order.
			marker := filepath.Join(dir, "dimensions")
			if err := pty.Setsize(master, &pty.Winsize{Cols: 72, Rows: 18}); err != nil {
				t.Fatal(err)
			}
			_ = cmd.Process.Signal(syscall.SIGWINCH)
			// Observe a frame at the new size before querying Vim's dimensions.
			time.Sleep(100 * time.Millisecond)
			write(":call writefile([printf('%d,%d', &columns, &lines)], '" + marker + "')\r")
			waitFor(t, "resize reaches Vim", func() bool { b, _ := os.ReadFile(marker); return string(b) == "72,15\n" })
			if b, _ := os.ReadFile(file); string(b) != "hello é\n" {
				t.Fatalf("preview input leaked: %q", b)
			}
			if signalExit {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			} else {
				write("\x1cq")
			}
			wait := make(chan error, 1)
			go func() { wait <- cmd.Wait() }()
			select {
			case err := <-wait:
				if err != nil {
					t.Fatal("outer TUI failed", err)
				}
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Signal(syscall.SIGQUIT)
				select {
				case <-wait:
				case <-time.After(time.Second):
				}
				_ = master.Close()
				<-readerDone
				t.Fatal("TUI did not cleanly exit")
			}
			<-readerDone
			mu.Lock()
			defer mu.Unlock()
			if !strings.Contains(output.String(), "\x1b[?1049l") {
				t.Fatal("alternate screen was not restored")
			}
			if strings.LastIndex(output.String(), "\x1b_Ga=d") < strings.LastIndex(output.String(), "\x1b_Ga=T") {
				t.Fatal("exit left preview image behind")
			}
		})
	}
}

// This validates real PNG bytes in the host stream, not a placeholder status.
// It deliberately does not claim Ghostty/Kitty graphics interoperability.
func decodeHostImages(t *testing.T, output string) []image.Rectangle {
	t.Helper()
	var payload strings.Builder
	var images []image.Rectangle
	active := false
	for _, part := range strings.Split(output, "\x1b_G")[1:] {
		body, _, ok := strings.Cut(part, "\x1b\\")
		if !ok {
			continue
		}
		control, data, ok := strings.Cut(body, ";")
		if !ok {
			continue
		}
		if strings.Contains(control, "a=T") {
			payload.Reset()
			active = true
		}
		if !active {
			continue
		}
		payload.WriteString(data)
		if strings.Contains(control, "m=0") {
			decoded, err := base64.StdEncoding.DecodeString(payload.String())
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(decoded))
			if err != nil {
				t.Fatal(err)
			}
			images = append(images, img.Bounds())
			active = false
		}
	}
	return images
}

func TestNonInteractiveFailsBeforeVim(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "redirected")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := Run(context.Background(), Options{File: "score.ly"}, f, f); err == nil {
		t.Fatal("accepted redirected input/output")
	}
}
