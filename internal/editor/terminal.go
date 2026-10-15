package editor

import (
	"io"
	"sync"

	"github.com/charmbracelet/x/vt"
)

// Terminal owns the emulator. A snapshot and its cursor are captured under one
// lock, so the UI never combines cells and a cursor from different frames.
type Terminal struct {
	mu            sync.Mutex
	em            *vt.Emulator
	response      io.Closer
	responsesDone chan struct{}
	closeOnce     sync.Once
}

type Snapshot struct {
	Content       string
	X, Y          int
	Hidden, Blink bool
	Style         vt.CursorStyle
}

func newTerminal(cols, rows int, reply func([]byte) error) *Terminal {
	em := vt.NewEmulator(cols, rows)
	t := &Terminal{em: em, response: em.InputPipe().(io.Closer), responsesDone: make(chan struct{})}
	// Query replies must be drained independently: emulator.Write can itself
	// synchronously write a response into this pipe while holding mu.
	go func() {
		defer close(t.responsesDone)
		buf := make([]byte, 4096)
		for {
			n, err := em.Read(buf)
			if n > 0 {
				_ = reply(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	return t
}

func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.em.Write(p)
}

func (t *Terminal) Resize(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.em.Resize(cols, rows)
}

type Mouse = vt.Mouse

func (t *Terminal) SendMouse(event Mouse) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.em.SendMouse(event)
}

func (t *Terminal) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.em.CursorPosition()
	style, blink := t.em.CursorStyle()
	return Snapshot{Content: t.em.Render(), X: p.X, Y: p.Y,
		Hidden: t.em.CursorHidden(), Style: style, Blink: blink}
}

func (t *Terminal) Close() {
	t.closeOnce.Do(func() {
		// Close only the response pipe first. The pinned emulator checks an
		// unsynchronized closed field in Read; join that reader before Close.
		_ = t.response.Close()
		<-t.responsesDone
		t.mu.Lock()
		defer t.mu.Unlock()
		_ = t.em.Close()
	})
}
