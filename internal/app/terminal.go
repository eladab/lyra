//go:build darwin || linux

package app

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// prepareTerminal keeps input raw while matching Bubble Tea's output contract.
// WithInput(nil) makes the pinned renderer assume the terminal maps LF to CRLF.
// MakeRaw disables that mapping, causing its optimized cursor moves to drift.
func prepareTerminal(in, out *os.File) (func() error, error) {
	outputState, err := term.GetState(int(out.Fd()))
	if err != nil {
		return nil, fmt.Errorf("read terminal output settings: %w", err)
	}
	inputState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return nil, fmt.Errorf("enter terminal raw mode: %w", err)
	}
	restore := func() error {
		var result error
		// Save both states before configuring output: stdin and stdout usually
		// share a terminal, but Run also accepts distinct terminal devices.
		if err := term.Restore(int(out.Fd()), outputState); err != nil {
			result = fmt.Errorf("restore terminal output: %w", err)
		}
		if err := term.Restore(int(in.Fd()), inputState); err != nil {
			result = errors.Join(result, fmt.Errorf("restore terminal input: %w", err))
		}
		return result
	}
	mode, err := unix.IoctlGetTermios(int(out.Fd()), termiosGetRequest)
	if err == nil {
		// Enable only newline mapping; retain raw input, and avoid inherited
		// tab expansion, CR translation or other output transformations.
		mode.Oflag = unix.OPOST | unix.ONLCR
		err = unix.IoctlSetTermios(int(out.Fd()), termiosSetRequest, mode)
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("configure terminal output: %w", err), restore())
	}
	return restore, nil
}
