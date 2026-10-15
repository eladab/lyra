//go:build darwin || linux

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"lyra/internal/editor"
	"lyra/internal/input"
	"lyra/internal/preview"
)

func renderModel(t *testing.T) *model {
	t.Helper()
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("requires traditional Vim")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte("set nocompatible noswapfile viminfo=\n"), 0600); err != nil {
		t.Fatal(err)
	}
	l := measure(120, 32)
	s, err := editor.Start(editor.Options{File: filepath.Join(dir, "score.ly"), Cols: l.editorCols, Rows: l.editorRows, Env: []string{"HOME=" + dir, "PATH=" + os.Getenv("PATH")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	f := newFrameRenderer(preview.Kitty{ID: 12345})
	return &model{session: s, layout: l, frames: f, focus: input.Editor, split: .5, zoom: 1, page: 1, graphics: true, cellWidth: 10, cellHeight: 20, previewState: preview.State{Payload: "AA==", Page: 1, Pages: 2}}
}

func TestGraphicsFramesClipClearResizeAndRestore(t *testing.T) {
	m := renderModel(t)
	first := m.frames.frame(m)
	if !strings.Contains(first, "\x1b[4;61H\x1b_Ga=T") || !strings.Contains(first, "c=60,r=28,C=1") {
		t.Fatalf("wrong preview bounds: %q", first)
	}
	if strings.Index(first, "SCORE") > strings.Index(first, "\x1b_Ga=T") {
		t.Fatal("image must follow text frame")
	}
	second := m.frames.frame(m)
	if strings.Contains(second, "a=T") {
		t.Fatal("unchanged image retransmitted on editor frame")
	}
	m.diagnostics = true
	diagnostics := m.frames.frame(m)
	if !strings.Contains(diagnostics, "a=d,d=I,i=12345") || strings.Contains(diagnostics, "a=T") {
		t.Fatal("diagnostics did not clear owned image")
	}
	m.diagnostics = false
	m.frames.frame(m)
	m.redraw = true
	redraw := m.frames.frame(m)
	if !strings.Contains(redraw, "\x1b[2J") || !strings.Contains(redraw, "a=T") {
		t.Fatal("full redraw did not restore image")
	}
	if err := m.resizeSplit(.65); err != nil {
		t.Fatal(err)
	}
	resized := m.frames.frame(m)
	if m.layout.editorCols != 77 || !strings.Contains(resized, "\x1b[4;79H\x1b_Ga=T") || !strings.Contains(resized, "c=42,r=28,C=1") {
		t.Fatal("adjusted split did not move/clip image")
	}
	// Focus changes on a narrow terminal never change the editor PTY geometry.
	m.layout = measureSplit(72, 18, m.split)
	m.focus = input.Editor
	if frame := m.frames.frame(m); strings.Contains(frame, "a=T") {
		t.Fatal("hidden narrow preview still placed an image")
	}
	m.focus = input.Preview
	if frame := m.frames.frame(m); !strings.Contains(frame, "\x1b[4;1H\x1b_Ga=T") || !strings.Contains(frame, "c=72,r=14,C=1") {
		t.Fatal("narrow preview did not fit content bounds")
	}
	m.layout = measureSplit(1, 1, m.split)
	if frame := m.frames.frame(m); strings.Contains(frame, "a=T") {
		t.Fatal("tiny terminal placed an image")
	}
	cleanup := m.frames.cleanup()
	for _, want := range []string{"a=d,d=I,i=12345", "\x1b[?1002l", "\x1b[?1006l", "\x1b[?2004l", "\x1b[?1049l"} {
		if !strings.Contains(cleanup, want) {
			t.Fatalf("cleanup missing %q", want)
		}
	}
}
func TestFrameAcknowledgmentsDoNotAcknowledgeQueries(t *testing.T) {
	m := &model{frames: newFrameRenderer(preview.Kitty{ID: 12}), framePending: true}
	m.Update(tea.RawMsg{Msg: "\x1b[16t"})
	if !m.framePending {
		t.Fatal("query acknowledged an unrelated in-flight frame")
	}
	m.dirty = false
	m.Update(tea.RawMsg{Msg: frameOutput("frame")})
	if m.framePending {
		t.Fatal("frame was not acknowledged")
	}
}

func TestPreviewFitAndZoomControls(t *testing.T) {
	m := renderModel(t)
	m.previewKeys([]byte("++jjll"))
	if m.fullPage || m.zoom <= 1 || m.scrollX == 0 || m.scrollY == 0 {
		t.Fatal("content zoom/pan controls failed")
	}
	m.previewKeys([]byte("f"))
	if !m.fullPage || m.zoom != 1 || m.scrollX != 0 || m.scrollY != 0 {
		t.Fatal("full page fit did not reset zoom and scroll")
	}
	m.previewKeys([]byte("+0"))
	if !m.fullPage || m.zoom != 1 {
		t.Fatal("reset did not preserve fit mode")
	}
	m.previewKeys([]byte("f"))
	if m.fullPage || m.zoom != 1 {
		t.Fatal("cannot return to content fit")
	}
}

func TestPreviewThemeChangesPreserveViewportAndEditorFocus(t *testing.T) {
	m := renderModel(t)
	m.previewTheme = preview.ThemeWarm
	m.zoom, m.page, m.scrollX, m.scrollY = 1.5, 2, 30, 40
	wantLayout := m.layout
	if err := m.dispatch([]input.Event{{Action: input.PreviewTheme}}); err != nil {
		t.Fatal(err)
	}
	if m.previewTheme != preview.ThemeNight || m.focus != input.Editor || m.layout != wantLayout {
		t.Fatal("prefix theme action changed focus or layout")
	}
	m.previewKeys([]byte("tt"))
	if m.previewTheme != preview.ThemeWarm || m.zoom != 1.5 || m.page != 2 || m.scrollX != 30 || m.scrollY != 40 {
		t.Fatal("theme cycle changed zoom, page or scroll")
	}
}
func TestSplitKeyboardAndMouseGeometry(t *testing.T) {
	m := renderModel(t)
	if m.layout.editorCols != 59 || m.layout.previewCols != 60 {
		t.Fatal("split does not start at half width")
	}
	for i := 0; i < 30; i++ {
		if err := m.dispatch([]input.Event{{Action: input.Wider}}); err != nil {
			t.Fatal(err)
		}
	}
	if m.layout.previewCols < 20 {
		t.Fatal("keyboard resize collapsed preview")
	}
	if err := m.dispatch([]input.Event{{Action: input.EqualSplit}, {Action: input.ToggleMouse}, {Action: input.MouseEvent, Code: 0, X: 59, Y: 10}, {Action: input.MouseEvent, Code: 32, X: 70, Y: 10}, {Action: input.MouseEvent, Code: 0, X: 70, Y: 10, Release: true}}); err != nil {
		t.Fatal(err)
	}
	if m.dragging || m.layout.editorCols != 70 {
		t.Fatalf("divider drag/release failed: %+v", m.layout)
	}
	if err := m.dispatch([]input.Event{{Action: input.MouseEvent, Code: 65, X: 90, Y: 10}}); err != nil {
		t.Fatal(err)
	}
	if m.focus != input.Preview || m.scrollY != 60 {
		t.Fatal("preview wheel did not scroll")
	}
	before := m.layout
	m.dispatch([]input.Event{{Action: input.Editor}, {Action: input.Preview}})
	if m.layout != before {
		t.Fatal("focus changed editor dimensions")
	}
}
