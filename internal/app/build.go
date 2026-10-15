//go:build darwin || linux

package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) waitBuild() tea.Cmd {
	if m.builds == nil {
		return nil
	}
	s := m.builds
	return func() tea.Msg {
		select {
		case <-s.Changed():
			return buildMsg(s.Snapshot())
		case <-s.Done():
			return nil
		}
	}
}

func (m *model) paneWidth() int {
	if m.layout.previewCols > 0 {
		return m.layout.previewCols
	}
	return m.layout.width
}

func (m *model) buildLines(width int) []string {
	s := m.buildState
	status := s.Status
	if status == "" {
		status = "Starting"
	}
	lines := []string{"LilyPond builds", "", "Target: " + m.file, "Launch score stays the target.", "", fmt.Sprintf("%s · generation %d", status, s.Generation)}
	if m.buildErr != "" {
		lines = append(lines, "Build service unavailable:", m.buildErr)
	}
	if s.Latest != nil {
		if s.Latest.Err != nil {
			lines = append(lines, s.Latest.Err.Error())
		} else {
			lines = append(lines, fmt.Sprintf("Finished in %s", s.Latest.Duration.Round(10*time.Millisecond)))
		}
	}
	if s.Latest != nil && !m.showLog && len(s.Latest.Diagnostics) > 0 {
		lines = append(lines, "", "Diagnostics:")
		for _, diagnostic := range s.Latest.Diagnostics {
			location := diagnostic.File
			if diagnostic.Line > 0 {
				location += fmt.Sprintf(":%d:%d", diagnostic.Line, diagnostic.Column)
			}
			lines = append(lines, location, diagnostic.Severity+": "+diagnostic.Message)
		}
	}
	if s.LastGood != nil {
		good := s.LastGood
		lines = append(lines, "", fmt.Sprintf("Last good: generation %d", good.Generation))
		for _, path := range good.PDFs {
			lines = append(lines, "PDF: "+path)
		}
		for _, path := range good.MIDIs {
			lines = append(lines, "MIDI: "+path)
		}
		if len(good.PDFs) == 0 {
			lines = append(lines, "No PDF: add a layout block.")
		}
		if len(good.MIDIs) == 0 {
			lines = append(lines, "No MIDI: add a MIDI block.")
		}
		lines = append(lines, "Outputs kept for this session.")
	}
	lines = append(lines, "", "Ctrl+\\ p  Engraved PDF preview", "Ctrl+\\ a  Playback and audio setup", "", "Ctrl+\\ b  Rebuild saved score", "Ctrl+\\ c  Cancel build", "j/k scroll · l toggle raw log")
	if s.WatchWarning != "" {
		lines = append(lines, "", s.WatchWarning)
	}
	if s.Latest != nil {
		if m.showLog {
			lines = append(lines, "", "Compiler log:", s.Latest.Log)
			if s.Latest.Truncated {
				lines = append(lines, "[Log truncated at 256 KiB]")
			}
		} else if len(s.Latest.Diagnostics) == 0 {
			lines = append(lines, "", "No source diagnostics. Toggle l for log.")
		}
	}
	// Compiler output and source paths are untrusted text. Remove controls before
	// wrapping so they cannot move the host cursor or alter Lyra's chrome.
	var wrapped []string
	for _, line := range lines {
		for _, row := range strings.Split(line, "\n") {
			wrapped = append(wrapped, strings.Split(ansi.Hardwrap(cleanLabel(row), max(1, width), true), "\n")...)
		}
	}
	return wrapped
}
