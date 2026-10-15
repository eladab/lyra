//go:build darwin || linux

package app

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lyra/internal/audio"
	"lyra/internal/editor"
	"lyra/internal/input"
	"lyra/internal/preview"
)

// Optional offscreen visual evidence, using real Vim and real engraved pages.
// This does not qualify any native terminal's graphics interoperability.
func TestAppearanceArtifacts(t *testing.T) {
	out := os.Getenv("LYRA_APPEARANCE_ARTIFACTS")
	if out == "" {
		t.Skip("set LYRA_APPEARANCE_ARTIFACTS to export styled terminal rows and preview PNGs")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	score, err := os.ReadFile("../testdata/scale.ly")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "playback.ly")
	if err := os.WriteFile(source, []byte(strings.ReplaceAll(string(score), "Lyra integration score", "Lyra playback")), 0600); err != nil {
		t.Fatal(err)
	}
	rc := "set nocompatible number termguicolors laststatus=2 noswapfile viminfo=\nsyntax enable\nset shortmess+=I\nhi Normal guifg=#d8dee9 guibg=#23292e\nhi EndOfBuffer guifg=#46505b guibg=#23292e\nhi LineNr guifg=#768292 guibg=#23292e\n"
	if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}
	l := measure(140, 42)
	s, err := editor.Start(editor.Options{File: source, Cols: l.editorCols, Rows: l.editorRows, Env: []string{"HOME=" + dir, "PATH=" + os.Getenv("PATH")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	waitFor(t, "syntax-painted Vim", func() bool { return strings.Contains(s.Terminal.Snapshot().Content, "midi") })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lilypond", "--output="+filepath.Join(dir, "score"), source)
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("engrave visual fixture: %v %s", err, output)
	}
	p, err := preview.NewService(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	m := &model{session: s, layout: l, file: source, focus: input.Editor, frames: newFrameRenderer(preview.Kitty{ID: 1}), graphics: true, cellWidth: 12, cellHeight: 24, zoom: 1, page: 1,
		audio: &audio.Service{}, audioState: audio.State{Status: "Ready", Duration: 14.9, Volume: 70}}
	for i, theme := range []preview.Theme{preview.ThemeWarm, preview.ThemeNight, preview.ThemePaper} {
		p.Submit(preview.Request{Version: uint64(i + 1), Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(filepath.Join(dir, "score.pdf")) }, Page: 1, Width: l.previewCols * 12, Height: (l.editorRows - 1) * 24, Zoom: 1, Theme: theme})
		waitFor(t, "themed engraved page", func() bool { return p.Snapshot().Version == uint64(i+1) })
		m.previewState, m.previewTheme = p.Snapshot(), theme
		if m.previewState.Err != nil {
			t.Fatal(m.previewState.Err)
		}
		png, err := base64.StdEncoding.DecodeString(m.previewState.Payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, string(theme)+"-score.png"), png, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, string(theme)+".ansi"), []byte(m.View().Content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
