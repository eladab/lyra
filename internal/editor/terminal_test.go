package editor

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestEmulationAndQueryReplies(t *testing.T) {
	replies := make(chan string, 8)
	term := newTerminal(12, 4, func(p []byte) error { replies <- string(p); return nil })
	defer term.Close()
	_, _ = term.Write([]byte("base\x1b[?1049h\x1b[2J\x1b[2;3H\x1b[38;2;12;34;56mCafé\x1b[0m\x1b[6n"))
	s := term.Snapshot()
	if !strings.Contains(ansi.Strip(s.Content), "Café") || s.X != 6 || s.Y != 1 {
		t.Fatalf("bad screen: %+v", s)
	}
	if !strings.Contains(s.Content, "12;34;56") {
		t.Fatalf("lost RGB style: %q", s.Content)
	}
	select {
	case reply := <-replies:
		if reply != "\x1b[2;7R" {
			t.Fatalf("bad DSR: %q", reply)
		}
	case <-time.After(time.Second):
		t.Fatal("emulator did not answer DSR")
	}
	_, _ = term.Write([]byte("\x1b[?1049l"))
	if !strings.Contains(ansi.Strip(term.Snapshot().Content), "base") {
		t.Fatal("alternate screen did not restore main screen")
	}
}

func TestConcurrentScreenResizeAndSnapshot(t *testing.T) {
	term := newTerminal(80, 24, func([]byte) error { return nil })
	defer term.Close()
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 120 {
			_, _ = term.Write([]byte("\x1b[Hhello\x1b[2;3Hworld\x1b[?25l\x1b[?25h"))
		}
	})
	wg.Go(func() {
		for i := range 120 {
			term.Resize(40+i%5, 12+i%3)
		}
	})
	wg.Go(func() {
		for range 120 {
			_ = term.Snapshot()
		}
	})
	wg.Wait()
}
