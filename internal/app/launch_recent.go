//go:build darwin || linux

package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

type launchRecents struct {
	version uint64
	paths   []string
	err     error
}

func (m *launcher) loadRecent() tea.Cmd {
	m.version++
	version, ctx, recent := m.version, m.ctx, m.recent
	m.page, m.creating = launchRecent, false
	m.entries, m.selected, m.loading = nil, 0, true
	m.err, m.notice = "", ""
	return func() tea.Msg {
		paths, err := recent.Load(ctx)
		return launchRecents{version: version, paths: paths, err: err}
	}
}

func (m *launcher) recentKey(key uv.KeyPressEvent) tea.Cmd {
	switch key.String() {
	case "esc", "left", "backspace", "ctrl+h":
		m.version++
		m.page, m.loading, m.err = launchMenu, false, ""
	case "q":
		return tea.Quit
	case "up", "k":
		m.selected = max(0, m.selected-1)
	case "down", "j":
		m.selected = min(max(0, len(m.entries)-1), m.selected+1)
	case "pgup":
		m.selected = max(0, m.selected-m.recentRows())
	case "pgdown":
		m.selected = min(max(0, len(m.entries)-1), m.selected+m.recentRows())
	case "home", "g":
		m.selected = 0
	case "end", "G":
		m.selected = max(0, len(m.entries)-1)
	case "enter", "right":
		if !m.loading && len(m.entries) > 0 {
			return m.choose(m.entries[m.selected].path, false)
		}
	}
	return nil
}

func (m *launcher) setRecent(msg launchRecents) {
	if msg.version != m.version {
		return
	}
	m.loading = false
	if msg.err != nil {
		m.err = "Read recent scores: " + msg.err.Error()
		return
	}
	for _, path := range msg.paths {
		m.entries = append(m.entries, scoreEntry{name: filepath.Base(path), path: path})
	}
}
