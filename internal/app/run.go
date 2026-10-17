//go:build darwin || linux

package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
	"lyra/internal/audio"
	documentation "lyra/internal/docs"
	"lyra/internal/editor"
	"lyra/internal/input"
	"lyra/internal/lilypond"
	"lyra/internal/preview"
)

type Options struct {
	Vim, File, LilyPond                     string
	IncludeDirs                             []string
	BuildTimeout                            time.Duration
	PDFInfo, PDFToPPM                       string
	FluidSynth, MPV, SoundFont, AudioOutput string
	PreviewTheme                            preview.Theme
	DisableLilyPondSyntax                   bool
	DocsVersion                             string
	DisableDocsKey                          bool
}

func Run(ctx context.Context, opts Options, in, out *os.File) (result error) {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return errors.New("Lyra needs an interactive terminal for both input and output")
	}
	if opts.File == "" {
		path, err := selectScore(ctx, in, out)
		if err != nil || path == "" {
			return err
		}
		opts.File = path
	}
	width, height, err := term.GetSize(int(out.Fd()))
	if err != nil {
		return fmt.Errorf("read terminal size: %w", err)
	}
	l := measure(width, height)
	builds, buildErr := lilypond.NewService(ctx, lilypond.Options{Source: opts.File, Executable: opts.LilyPond, IncludeDirs: opts.IncludeDirs, Timeout: opts.BuildTimeout})
	notification := ""
	if builds != nil {
		defer builds.Close()
		notification = builds.SaveNotification
	}
	docsDir, err := os.MkdirTemp("", "lyra-docs-")
	if err != nil {
		return fmt.Errorf("create documentation bridge: %w", err)
	}
	defer os.RemoveAll(docsDir)
	docsNotification := filepath.Join(docsDir, "lookup.json")
	session, err := editor.Start(editor.Options{Vim: opts.Vim, File: opts.File, Cols: l.editorCols, Rows: l.editorRows, SaveNotification: notification, DisableLilyPondSyntax: opts.DisableLilyPondSyntax, DocsNotification: docsNotification, DisableDocsKey: opts.DisableDocsKey})
	if err != nil {
		return err
	}
	defer session.Close()
	restore, err := prepareTerminal(in, out)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, restore())
	}()
	reader, err := newRawReader(in)
	if err != nil {
		return fmt.Errorf("open terminal input: %w", err)
	}
	defer reader.Close()
	previews, previewErr := preview.NewService(ctx, opts.PDFInfo, opts.PDFToPPM)
	if previews != nil {
		defer previews.Close()
	}
	id := rand.Uint32() | 0x1000000
	frames := newFrameRenderer(preview.Kitty{ID: id})
	m := &model{session: session, reader: reader, layout: l, file: opts.File, focus: input.Editor, builds: builds, previews: previews, frames: frames, imageID: id, split: .5, zoom: 1, page: 1, previewTheme: opts.PreviewTheme}
	docsCtx, stopDocs := context.WithCancel(ctx)
	defer stopDocs()
	m.docs = docsPane{ctx: docsCtx, loader: documentation.Loader{Version: opts.DocsVersion}, notification: docsNotification, section: -1}
	playback, audioErr := audio.NewService(ctx, audio.Options{FluidSynth: opts.FluidSynth, MPV: opts.MPV, SoundFont: opts.SoundFont, Output: opts.AudioOutput})
	if playback != nil {
		defer playback.Close()
		m.audio = playback
		m.audioState = playback.Snapshot()
	} else {
		m.audioErr = audioErr.Error()
	}
	if previewErr != nil {
		m.previewErr = previewErr.Error()
	}
	if buildErr != nil {
		m.buildErr = buildErr.Error()
	} else {
		m.buildState = builds.Snapshot()
		m.updateAudioSource()
	}
	// WithInput(nil) prevents Bubble Tea's key parser and capability queries
	// from competing for bytes with Vim. Lyra owns raw mode and cancellation.
	// The CLI context owns termination signals. A second Bubble Tea handler can
	// block sending Quit after context cancellation has stopped the event loop.
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(out), tea.WithContext(ctx), tea.WithWindowSize(width, height), tea.WithoutSignalHandler(), tea.WithoutRenderer())
	// WithoutRenderer also disables Bubble Tea's tty discovery. Keep its event
	// loop as output owner while explicitly routing kernel window changes.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	resizeCtx, stopResize := context.WithCancel(ctx)
	resizeDone := make(chan struct{})
	go func() {
		defer close(resizeDone)
		for {
			select {
			case <-resizeCtx.Done():
				return
			case <-winch:
				w, h, err := term.GetSize(int(out.Fd()))
				if err == nil {
					p.Send(tea.WindowSizeMsg{Width: w, Height: h})
				}
			}
		}
	}()
	defer func() { stopResize(); signal.Stop(winch); <-resizeDone }()
	_, err = p.Run()
	// The program has joined its output loop. Restore owned graphics and modes
	// before restoring termios, including context cancellation and child death.
	_, cleanupErr := out.WriteString(frames.cleanup())
	return errors.Join(m.err, err, cleanupErr)
}
