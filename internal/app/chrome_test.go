//go:build darwin || linux

package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"lyra/internal/input"
)

func TestChromeFitsWideNamesAndFocusAtEverySize(t *testing.T) {
	m := renderModel(t)
	m.file = "/scores/夜の音楽 — a long composition name with spaces.ly"
	for _, width := range []int{1, 2, 6, 20, 60, 87, 88, 120, 180} {
		m.layout = measure(width, 24)
		for _, focus := range []input.Action{input.Editor, input.Preview} {
			m.focus = focus
			for row, line := range strings.Split(m.View().Content, "\n") {
				if got := ansi.StringWidth(line); got != width {
					t.Fatalf("width=%d focus=%d row=%d: got %d cells in %q", width, focus, row, got, line)
				}
			}
		}
	}
	m.layout = measure(120, 24)
	m.focus = input.Editor
	if !strings.Contains(ansi.Strip(m.paneBars()), "● VIM") || !strings.Contains(ansi.Strip(m.paneBars()), "○ SCORE") {
		t.Fatal("editor focus indicator missing")
	}
	m.focus = input.Preview
	if !strings.Contains(ansi.Strip(m.paneBars()), "○ VIM") || !strings.Contains(ansi.Strip(m.paneBars()), "● SCORE") {
		t.Fatal("preview focus indicator missing")
	}
}
