//go:build darwin || linux

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
)

type draftSavedMsg struct {
	path string
	err  error
}

type adoptionWatch struct {
	events chan draftSavedMsg
	done   chan struct{}
	cancel context.CancelFunc
}

// Vim atomically publishes one JSON path after a successful first write. Keep
// file I/O off the event loop and use the same channel until adoption or exit.
func watchAdoption(parent context.Context, path string) *adoptionWatch {
	ctx, cancel := context.WithCancel(parent)
	w := &adoptionWatch{events: make(chan draftSavedMsg, 1), done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(w.done)
		defer close(w.events)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		lastError := ""
		for {
			name, err := readAdoption(path)
			if err == nil {
				select {
				case w.events <- draftSavedMsg{path: name}:
				case <-ctx.Done():
				}
				return
			}
			if !os.IsNotExist(err) && err.Error() != lastError {
				select {
				case w.events <- draftSavedMsg{err: err}:
				case <-ctx.Done():
					return
				}
				lastError = err.Error()
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return w
}

func (w *adoptionWatch) Close() { w.cancel(); <-w.done }

func readAdoption(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return "", err
	}
	var name string
	if len(data) > 64<<10 || json.Unmarshal(data, &name) != nil || name == "" || !filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid first-save notification")
	}
	return name, nil
}

func (m *model) waitAdoption() tea.Cmd {
	if m.adoption == nil {
		return nil
	}
	w := m.adoption
	return func() tea.Msg {
		msg, ok := <-w.events
		if !ok {
			return nil
		}
		return msg
	}
}

func sameFile(a, b string) bool {
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	return err == nil && os.SameFile(left, right)
}
