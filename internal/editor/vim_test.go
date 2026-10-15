//go:build darwin || linux

package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestResizeDuringChildExit(t *testing.T) {
	s, _ := testVim(t)
	var resizing sync.WaitGroup
	resizing.Go(func() {
		for i := 0; i < 100; i++ {
			_ = s.Resize(64+i%15, 20+i%5)
		}
	})
	s.Close()
	resizing.Wait()
	if err := s.Resize(80, 24); err == nil {
		t.Fatal("resize accepted after child exit")
	}
}

func eventually(t *testing.T, description string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout: " + description)
}

func testVim(t *testing.T, notification ...string) (*Session, string) {
	t.Helper()
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("traditional Vim is required for PTY integration tests")
	}
	dir := t.TempDir()
	rc := "set nocompatible\nset noswapfile viminfo= nomore\nset shortmess+=I\nset ttimeoutlen=20\n" +
		"let g:lyra_test_config = 'loaded'\n" +
		"nnoremap <F5> iMapped<Esc>\n" +
		"autocmd VimEnter * call writefile(['ready'], '" + filepath.Join(dir, "ready") + "')\n"
	if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "HOME" && key != "VIMINIT" && key != "EXINIT" && key != "GVIMINIT" {
			env = append(env, item)
		}
	}
	env = append(env, "HOME="+dir)
	opts := Options{File: filepath.Join(dir, "-score with spaces.ly"), Cols: 64, Rows: 20, Env: env}
	if len(notification) > 0 {
		opts.SaveNotification = notification[0]
	}
	s, err := Start(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := os.ReadFile(filepath.Join(dir, "-score with spaces.ly"))
			t.Logf("saved=%q screen=%q", b, ansi.Strip(s.Terminal.Snapshot().Content))
		}
	})
	eventually(t, "Vim reading normal .vimrc", func() bool { _, err := os.Stat(filepath.Join(dir, "ready")); return err == nil })
	return s, dir
}

func send(t *testing.T, s *Session, data string) {
	t.Helper()
	if err := s.WriteInput([]byte(data)); err != nil {
		t.Fatal(err)
	}
}

func readEquals(path, want string) bool {
	b, err := os.ReadFile(path)
	return err == nil && string(b) == want
}

func TestRealVimEditingConfigResizeAndQuit(t *testing.T) {
	s, dir := testVim(t)
	file := filepath.Join(dir, "-score with spaces.ly")
	send(t, s, "iCafé notes\tlyra\x1b")
	// Visual mode deletes the first character, normal mode replaces the next.
	send(t, s, "0vld0iC\x1b:w\r")
	eventually(t, "saved UTF-8 and tab through genuine Vim", func() bool { return readEquals(file, "Cfé notes\tlyra\n") })
	if err := s.Resize(93, 31); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "dimensions")
	send(t, s, fmt.Sprintf(":call writefile([printf('%%d,%%d', &columns, &lines), g:lyra_test_config], '%s')\r", marker))
	eventually(t, "Vim receives SIGWINCH and keeps user configuration", func() bool { return readEquals(marker, "93,31\nloaded\n") })
	send(t, s, ":wq\r")
	select {
	case <-s.Done():
		if err := s.Err(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Vim was not reaped")
	}
	s.Close() // safe and idempotent after ordinary exit
}

func TestQuitPreservesUnsavedConfirmation(t *testing.T) {
	s, dir := testVim(t)
	send(t, s, "iunsaved\x1b")
	if err := s.RequestQuit(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "Vim unsaved-buffer prompt", func() bool { return strings.Contains(ansi.Strip(s.Terminal.Snapshot().Content), "Save changes") })
	select {
	case <-s.Done():
		t.Fatal("quit discarded unsaved changes")
	default:
	}
	send(t, s, "c")
	eventually(t, "cancel confirmation returns to editor", func() bool {
		return !strings.Contains(ansi.Strip(s.Terminal.Snapshot().Content), "Save changes")
	})
	send(t, s, ":wq\r")
	eventually(t, "cancel then save", func() bool { return readEquals(filepath.Join(dir, "-score with spaces.ly"), "unsaved\n") })
}

func TestVimArrowKeysPasteAndUserMapping(t *testing.T) {
	s, dir := testVim(t)
	file := filepath.Join(dir, "-score with spaces.ly")
	send(t, s, "\x1b[15~:w\r")
	eventually(t, "user F5 mapping", func() bool { return readEquals(file, "Mapped\n") })
	send(t, s, "A!\x1b\x1b[Dr?:w\r")
	eventually(t, "raw arrow key", func() bool { return readEquals(file, "Mappe?!\n") })
	send(t, s, "gg0i\x1b[200~pasted\nnotes\x1b[201~\x1b:w\r")
	eventually(t, "bracketed paste", func() bool { return readEquals(file, "pasted\nnotesMappe?!\n") })
	send(t, s, ":q\r")
}

func TestSaveNotificationOnlyFollowsLaunchScore(t *testing.T) {
	notification := filepath.Join(t.TempDir(), "saved")
	s, dir := testVim(t, notification)
	send(t, s, "iscore\x1b:w\r")
	eventually(t, "session save hook", func() bool { _, err := os.Stat(notification); return err == nil })
	first, err := os.ReadFile(notification)
	if err != nil {
		t.Fatal(err)
	}
	send(t, s, ":w\r")
	eventually(t, "unchanged save is still notified", func() bool { data, _ := os.ReadFile(notification); return string(data) != string(first) })
	second, _ := os.ReadFile(notification)
	other := filepath.Join(dir, "other.ly")
	send(t, s, ":edit "+other+"\riother\x1b:w\r")
	eventually(t, "other Vim buffer saved normally", func() bool { return readEquals(other, "other\n") })
	// A marker after :w proves the synchronous BufWritePost handlers completed.
	marker := filepath.Join(dir, "hook-check")
	send(t, s, ":call writefile([g:lyra_test_config], '"+marker+"')\r")
	eventually(t, "user configuration intact", func() bool { return readEquals(marker, "loaded\n") })
	if data, _ := os.ReadFile(notification); string(data) != string(second) {
		t.Fatal("save hook followed another buffer")
	}
	send(t, s, ":q\r")
}

func TestUnexpectedDeathAndForcedClose(t *testing.T) {
	for _, kill := range []bool{false, true} {
		t.Run(fmt.Sprint(kill), func(t *testing.T) {
			s, _ := testVim(t)
			if kill {
				_ = s.cmd.Process.Kill()
			} else {
				s.Close()
			}
			select {
			case <-s.Done():
			case <-time.After(3 * time.Second):
				t.Fatal("child or IO goroutine leaked")
			}
			if kill && s.Err() == nil {
				t.Fatal("unexpected child failure was not reported")
			}
		})
	}
}

func TestRejectNeovimAndMissingVim(t *testing.T) {
	if _, err := ResolveVim(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("accepted missing executable")
	}
	fake := filepath.Join(t.TempDir(), "vim")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'NVIM v0.12'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveVim(fake); err == nil {
		t.Fatal("accepted Neovim under the name vim")
	}
}
