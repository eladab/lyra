//go:build darwin || linux

package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"golang.org/x/term"
)

// The startup program finishes and joins its sole reader before Vim starts.
// WithInput(nil) also disables host queries and keyboard protocol negotiation,
// so no chooser reply or alternate input mode can spill into the child editor.
func selectScore(ctx context.Context, in, out *os.File) (path string, result error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	w, h, err := term.GetSize(int(out.Fd()))
	if err != nil {
		return "", err
	}
	restore, err := prepareTerminal(in, out)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, restore()) }()
	reader, err := newRawReader(in)
	if err != nil {
		return "", err
	}
	inputCtx, stopInput := context.WithCancel(ctx)
	events := make(chan uv.Event, 16)
	inputDone := make(chan struct{})
	var inputErr error
	go func() {
		inputErr = uv.NewTerminalReader(reader, os.Getenv("TERM")).StreamEvents(inputCtx, events)
		close(events)
		close(inputDone)
	}()
	defer func() {
		stopInput()
		_ = reader.Close()
		// The decoder flushes partial input on shutdown. Drain it after the
		// event loop stops so its bounded channel cannot hold up the join.
		for range events {
		}
		<-inputDone
		result = errors.Join(result, inputErr)
	}()
	m := newLauncher(inputCtx, events, directory, w, h)
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(out), tea.WithContext(ctx), tea.WithWindowSize(w, h), tea.WithColorProfile(colorprofile.TrueColor), tea.WithoutSignalHandler())
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
				if w, h, err := term.GetSize(int(out.Fd())); err == nil {
					p.Send(tea.WindowSizeMsg{Width: w, Height: h})
				}
			}
		}
	}()
	defer func() { stopResize(); signal.Stop(winch); <-resizeDone }()
	_, err = p.Run()
	// Bubble Tea has stopped writing before cleanup and the editor's next run.
	_, cleanupErr := out.WriteString("\x1b[?2004l\x1b[0m\x1b[?25h")
	return m.chosen, errors.Join(err, m.inputErr, cleanupErr)
}
