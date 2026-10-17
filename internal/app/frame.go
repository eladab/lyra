//go:build darwin || linux

package app

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"lyra/internal/input"
	"lyra/internal/preview"
)

type frameTick struct{}
type probeTimeout struct{}
type frameOutput string

func (f frameOutput) String() string { return string(f) }

// frameRenderer uses the same public cell renderer as Bubble Tea, composing
// graphics AFTER its text diff. Only tea.Raw writes the complete frame. One
// frame may be in flight at a time; RawMsg acknowledges it on the event loop.
// No image escapes pass through the styled-cell parser or race a renderer flush.
type frameRenderer struct {
	buf           bytes.Buffer
	text          *uv.TerminalRenderer
	width, height int
	started       bool
	image         string
	x, cols, rows int
	mouse         bool
	backend       preview.Backend
}

func newFrameRenderer(backend preview.Backend) *frameRenderer {
	f := &frameRenderer{backend: backend}
	f.text = uv.NewTerminalRenderer(&f.buf, os.Environ())
	f.text.SetFullscreen(true)
	f.text.SetRelativeCursor(false)
	f.text.SetColorProfile(colorprofile.TrueColor)
	f.text.SetMapNewline(true)
	// Scrolling terminal rows also scrolls graphics. Use cell diffs instead.
	f.text.SetScrollOptim(false)
	return f
}

func (f *frameRenderer) frame(m *model) string {
	v := m.View()
	l := m.layout
	var b strings.Builder
	b.WriteString("\x1b[?2026h\x1b[?25l")
	if !f.started {
		b.WriteString("\x1b[?1049h\x1b[?2004h\x1b[0m\x1b[2J\x1b[H")
		f.text.SetPosition(0, 0)
		f.started = true
		b.WriteString(f.backend.Probe())
	}
	changed := l.width != f.width || l.height != f.height || m.redraw
	m.redraw = false
	if changed {
		f.text.Resize(l.width, l.height)
		f.text.Erase()
		f.width, f.height = l.width, l.height
	}
	x := 0
	if l.previewCols > 0 {
		x = l.editorCols + 1
	}
	cols, rows := m.paneWidth(), max(1, l.editorRows-1)
	payload := ""
	if m.graphics && !m.help && !m.diagnostics && !m.audioPane && !m.docs.visible && l.height >= 5 && (l.previewCols > 0 || m.focus == input.Preview) {
		payload = m.previewState.Payload
	}
	imageChanged := changed || payload != f.image || x != f.x || cols != f.cols || rows != f.rows
	if imageChanged && f.image != "" {
		b.WriteString(f.backend.Delete())
	}
	screen := uv.NewScreenBuffer(l.width, l.height)
	uv.NewStyledString(v.Content).Draw(screen, screen.Bounds())
	f.text.Render(screen.RenderBuffer)
	if v.Cursor != nil {
		f.text.MoveTo(v.Cursor.X, v.Cursor.Y)
	}
	_ = f.text.Flush() // destination is an in-memory bytes.Buffer
	b.Write(f.buf.Bytes())
	// ED2 clears terminal images too. Restore the viewport after any text
	// diff that uses it, even when the image content itself is unchanged.
	if bytes.Contains(f.buf.Bytes(), []byte("\x1b[2J")) {
		imageChanged = true
	}
	f.buf.Reset()
	if imageChanged && payload != "" {
		b.WriteString(f.backend.Draw(payload, x, 3, cols, rows))
	}
	f.image, f.x, f.cols, f.rows = payload, x, cols, rows
	if m.mouse != f.mouse {
		if m.mouse {
			b.WriteString("\x1b[?1002h\x1b[?1006h")
		} else {
			b.WriteString("\x1b[?1002l\x1b[?1006l")
		}
		f.mouse = m.mouse
	}
	if v.Cursor != nil {
		style := 1 + int(v.Cursor.Shape)*2
		if !v.Cursor.Blink {
			style++
		}
		fmt.Fprintf(&b, "\x1b[%d q\x1b[?25h", style)
	}
	b.WriteString("\x1b[?2026l")
	return b.String()
}

func (f *frameRenderer) cleanup() string {
	return f.backend.Delete() + "\x1b[?2026l\x1b[?1002l\x1b[?1006l\x1b[?2004l\x1b[0m\x1b[0 q\x1b[?25h\x1b[?1049l"
}

func (m *model) scheduleFrame() tea.Cmd {
	m.dirty = true
	if m.frames == nil || m.framePending {
		return nil
	}
	m.framePending = true
	return tea.Tick(time.Second/30, func(time.Time) tea.Msg { return frameTick{} })
}
