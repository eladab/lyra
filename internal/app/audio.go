//go:build darwin || linux

package app

import (
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"lyra/internal/audio"
	"lyra/internal/input"
)

type audioMsg audio.State

func (m *model) waitAudio() tea.Cmd {
	if m.audio == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-m.audio.Changed():
			return audioMsg(m.audio.Snapshot())
		case <-m.audio.Done():
			return nil
		}
	}
}
func (m *model) updateAudioSource() {
	if m.audio == nil || m.buildState.LastGood == nil {
		return
	}
	good := m.buildState.LastGood
	source := audio.Source{Generation: good.Generation}
	if len(good.MIDIs) > 0 {
		generation := good.Generation
		source.Open = func() (io.ReadCloser, error) { return m.builds.OpenMIDI(generation) }
	}
	m.audio.UpdateSource(source)
}
func (m *model) audioAction(action audio.Action) {
	if m.audio == nil {
		if m.audioErr == "" {
			m.audioErr = "Playback unavailable; open Ctrl+\\ a for setup"
		}
		return
	}
	if err := m.audio.Request(action); err != nil {
		m.audioErr = err.Error()
	} else {
		m.audioErr = ""
	}
}
func (m *model) audioKeys(data []byte) {
	keys := strings.NewReplacer("\x1b[D", ",", "\x1b[C", ".", "\x1b[H", "0").Replace(string(data))
	if strings.ContainsRune(keys, '\x1b') {
		return
	}
	for _, key := range keys {
		switch key {
		case 'q':
			m.focus = input.Editor
			return
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
		case '0':
			m.audioAction(audio.Restart)
		case '+', '=':
			m.audioAction(audio.Louder)
		case '-':
			m.audioAction(audio.Quieter)
		}
	}
}
func clockText(seconds float64) string {
	n := max(0, int(seconds))
	return fmt.Sprintf("%d:%02d", n/60, n%60)
}
func (m *model) audioSummary() string {
	if m.audio == nil {
		if m.audioErr != "" {
			return "Audio setup: Ctrl+\\ a"
		}
		return ""
	}
	s := m.audioState
	status := s.Status
	if status == "" {
		status = "Waiting for MIDI"
	}
	summary := fmt.Sprintf("%s %s/%s · %d%%", status, clockText(s.Position), clockText(s.Duration), s.Volume)
	if s.Preparing && s.Status != "Preparing audio" {
		summary += " · preparing"
	}
	if s.QueuedPlay {
		summary += " · play queued"
	}
	if s.Generation > 0 && s.ReadyGeneration > s.Generation {
		summary += " · u latest"
	}
	if s.Error != "" {
		summary += " · error: Ctrl+\\ a"
	}
	return cleanLabel(summary)
}
func (m *model) audioLines(width int) []string {
	s := m.audioState
	lines := []string{"Playback", m.audioSummary(), ""}
	if s.Duration > 0 {
		cells := max(1, width-2)
		position := min(cells, int(float64(cells)*s.Position/s.Duration))
		lines = append(lines, "["+strings.Repeat("=", position)+strings.Repeat("-", cells-position)+"]")
	}
	if s.Generation > 0 {
		lines = append(lines, fmt.Sprintf("Current take: %d", s.Generation))
	}
	if s.ReadyGeneration > s.Generation {
		lines = append(lines, fmt.Sprintf("New take %d ready · u to play", s.ReadyGeneration))
	}
	if !s.HasMIDI && m.buildState.LastGood != nil {
		lines = append(lines, "No MIDI in the latest score.", "Add \\midi { } inside \\score.")
	}
	if m.audioErr != "" {
		lines = append(lines, "", cleanLabel(m.audioErr))
	}
	if s.Error != "" {
		lines = append(lines, "", cleanLabel(s.Error))
	}
	lines = append(lines, "", "Space: play / pause", "s: stop · 0: restart", ",/.: seek back/forward 5s", "u: play latest take", "+/-: volume · q/Esc: Vim", "", "From Vim: Ctrl+\\ then", "Space play/pause · s stop", ",/. seek · u play latest", "a audio · p score preview")
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(cleanLabel(line), max(1, width), true), "\n")...)
	}
	return wrapped
}
