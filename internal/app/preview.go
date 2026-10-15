//go:build darwin || linux

package app

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"lyra/internal/audio"
	"lyra/internal/editor"
	"lyra/internal/input"
	"lyra/internal/preview"
)

type previewMsg preview.State

func (m *model) waitPreview() tea.Cmd {
	if m.previews == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-m.previews.Changed():
			return previewMsg(m.previews.Snapshot())
		case <-m.previews.Done():
			return nil
		}
	}
}
func (m *model) requestPreview() {
	good := m.buildState.LastGood
	if m.previews == nil || !m.graphics || m.cellWidth < 1 || good == nil || len(good.PDFs) == 0 || m.layout.height < 5 {
		return
	}
	m.previewVersion++
	generation := good.Generation
	m.previews.Submit(preview.Request{Version: m.previewVersion, Generation: generation,
		Open: func() (io.ReadCloser, error) { return m.builds.OpenPDF(generation) }, Page: max(1, m.page), Width: m.paneWidth() * m.cellWidth,
		Height: max(1, m.layout.editorRows-1) * m.cellHeight, Zoom: m.zoom, ScrollX: m.scrollX, ScrollY: m.scrollY, FullPage: m.fullPage, Theme: m.previewTheme})
	m.previewPending = true
}
func (m *model) hostReply(e input.Event) {
	if e.Code == 6 && e.X > 0 && e.Y > 0 && e.X <= 128 && e.Y <= 256 {
		m.cellWidth, m.cellHeight = e.X, e.Y
		m.requestPreview()
		return
	}
	if e.Code == 4 && m.cellWidth == 0 && m.layout.width > 0 && m.layout.height > 0 {
		m.cellWidth, m.cellHeight = max(1, e.X/m.layout.width), max(1, e.Y/m.layout.height)
		m.requestPreview()
		return
	}
	if strings.Contains(string(e.Data), fmt.Sprintf("i=%d;OK", m.imageID)) {
		m.graphics = true
		m.graphicsErr = ""
		m.requestPreview()
	}
}
func (m *model) resizeSplit(split float64) error {
	m.split = max(.1, min(.9, split))
	next := measureSplit(m.layout.width, m.layout.height, m.split)
	if next.editorCols != m.layout.editorCols {
		if err := m.session.Resize(next.editorCols, next.editorRows); err != nil {
			return err
		}
	}
	m.layout = next
	m.requestPreview()
	return nil
}
func (m *model) previewKeys(data []byte) {
	changed := false
	// Recognize complete keys even when several arrive in one raw read. Unknown
	// escapes and bracketed paste cannot become preview commands by accident.
	keys := strings.NewReplacer("\x1b[A", "k", "\x1b[B", "j", "\x1b[C", "l", "\x1b[D", "h", "\x1b[5~", "p", "\x1b[6~", "n").Replace(string(data))
	if strings.ContainsRune(keys, '\x1b') {
		return
	}
	for _, key := range keys {
		switch key {
		case 't':
			m.previewTheme = m.previewTheme.Next()
			changed = true
		case ' ':
			m.audioAction(audio.Toggle)
		case 's':
			m.audioAction(audio.Stop)
		case ',':
			m.audioAction(audio.Backward)
		case '.':
			m.audioAction(audio.Forward)
		case 'u':
			m.audioAction(audio.Latest)
		case 'q':
			m.focus = input.Editor
			return
		case 'n', ']':
			m.page = min(max(1, m.previewState.Pages), m.page+1)
			m.scrollX, m.scrollY = 0, 0
			changed = true
		case 'p', '[':
			m.page = max(1, m.page-1)
			m.scrollX, m.scrollY = 0, 0
			changed = true
		case '+', '=':
			m.zoom = min(4, m.zoom*1.25)
			changed = true
		case '-':
			m.zoom = max(.25, m.zoom/1.25)
			changed = true
		case '0':
			m.zoom = 1
			m.scrollX, m.scrollY = 0, 0
			changed = true
		case 'f':
			m.fullPage = !m.fullPage
			m.zoom = 1
			m.scrollX, m.scrollY = 0, 0
			changed = true
		case 'j':
			m.scrollY += m.cellHeight * 3
			changed = true
		case 'k':
			m.scrollY = max(0, m.scrollY-m.cellHeight*3)
			changed = true
		case 'h':
			m.scrollX = max(0, m.scrollX-m.cellWidth*3)
			changed = true
		case 'l':
			m.scrollX += m.cellWidth * 3
			changed = true
		case 'r':
			m.redraw = true
			changed = true
		case 'd':
			m.diagnostics = true
			m.paneScroll = 0
		}
	}
	if changed {
		m.requestPreview()
	}
}

func (m *model) cyclePreviewTheme() {
	m.previewTheme = m.previewTheme.Next()
	m.requestPreview()
}
func (m *model) previewStatus() string {
	if m.previewErr != "" {
		return m.previewErr
	}
	if m.graphicsErr != "" {
		return m.graphicsErr
	}
	if !m.graphics {
		return "Checking terminal graphics support…"
	}
	if m.cellWidth < 1 {
		return "Waiting for terminal cell dimensions…"
	}
	if m.previewState.Err != nil {
		return cleanLabel(m.previewState.Err.Error())
	}
	if m.previewState.Payload != "" {
		fit := "content"
		if m.previewState.FullPage {
			fit = "page"
		}
		status := fmt.Sprintf("Page %d/%d · %s · %.0f%% · %s", m.previewState.Page, m.previewState.Pages, fit, m.zoom*100, m.previewState.Theme.Label())
		if m.previewPending {
			status += " · updating"
		}
		if m.buildState.Status == "Failed" {
			status += " · last good; d diagnostics"
		}
		return status
	}
	if m.previewPending {
		return "Rendering engraved PDF…"
	}
	if m.buildErr != "" {
		return cleanLabel(m.buildErr)
	}
	if latest := m.buildState.Latest; latest != nil && latest.Err != nil {
		return cleanLabel(latest.Err.Error()) + " · Ctrl+\\ d: diagnostics"
	}
	if good := m.buildState.LastGood; good != nil && len(good.PDFs) == 0 {
		return "No PDF output: add a layout block · Ctrl+\\ d: builds"
	}
	if m.buildState.Status == "Building" || m.buildState.Status == "Queued" {
		return "Engraving saved score…"
	}
	return "No engraved PDF yet · :w to save · Ctrl+\\ b to build"
}

func (m *model) mouseEvent(e input.Event) error {
	if !m.mouse {
		return nil
	}
	wasDragging := m.dragging
	if e.Release {
		m.dragging = false
		if wasDragging {
			return nil
		}
	}
	if !e.Release && m.layout.previewCols > 0 && e.Code&67 == 0 && (m.dragging || e.X == m.layout.editorCols) {
		m.dragging = true
		return m.resizeSplit(float64(e.X) / float64(max(1, m.layout.width-1)))
	}
	inPreview := m.layout.previewCols > 0 && e.X > m.layout.editorCols || m.layout.previewCols == 0 && m.focus == input.Preview
	if e.Y < 2 || e.Y >= m.layout.height-1 {
		return nil
	}
	if inPreview {
		m.focus = input.Preview
		if m.audioPane {
			if e.Code&64 != 0 {
				key := []string{"+", "-", ",", "."}[e.Code&3]
				m.audioKeys([]byte(key))
			}
			return nil
		}
		if e.Code&64 != 0 {
			key := []string{"k", "j", "h", "l"}[e.Code&3]
			if m.diagnostics || m.help {
				if key == "j" {
					m.paneScroll += 3
				} else {
					m.paneScroll = max(0, m.paneScroll-3)
				}
			} else {
				m.previewKeys([]byte(key))
			}
		}
		return nil
	}
	if e.X < 0 || e.X >= m.layout.editorCols {
		return nil
	}
	m.focus = input.Editor
	// Let the emulator encode according to Vim's own requested mouse mode.
	button := uv.MouseLeft
	switch e.Code & 3 {
	case 1:
		button = uv.MouseMiddle
	case 2:
		button = uv.MouseRight
	}
	if e.Code&64 != 0 {
		button = []uv.MouseButton{uv.MouseWheelUp, uv.MouseWheelDown, uv.MouseWheelLeft, uv.MouseWheelRight}[e.Code&3]
	}
	mouse := uv.Mouse{X: e.X, Y: e.Y - 2, Button: button}
	if e.Code&4 != 0 {
		mouse.Mod |= uv.ModShift
	}
	if e.Code&8 != 0 {
		mouse.Mod |= uv.ModAlt
	}
	if e.Code&16 != 0 {
		mouse.Mod |= uv.ModCtrl
	}
	var event editor.Mouse = uv.MouseClickEvent(mouse)
	if e.Release {
		event = uv.MouseReleaseEvent(mouse)
	} else if e.Code&64 != 0 {
		event = uv.MouseWheelEvent(mouse)
	} else if e.Code&32 != 0 {
		event = uv.MouseMotionEvent(mouse)
	}
	m.session.Terminal.SendMouse(event)
	return nil
}

func probeTimer() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return probeTimeout{} })
}
