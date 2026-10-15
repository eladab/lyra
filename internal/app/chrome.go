//go:build darwin || linux

package app

import (
	"image/color"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"lyra/internal/input"
)

var (
	chromeBG     = lipgloss.Color("#202731")
	paneBG       = lipgloss.Color("#29313b")
	activePaneBG = lipgloss.Color("#2b3b40")
	chromeText   = lipgloss.Color("#e5e9f0")
	chromeMuted  = lipgloss.Color("#96a3b5")
	chromeAccent = lipgloss.Color("#b5a4e8")
	chromeFocus  = lipgloss.Color("#82c9c1")
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#465260"))
)

// bar composes exactly one terminal row, including wide filenames and tiny
// windows. Each segment is styled against the same background to avoid gaps.
func bar(left, right string, width int, background color.Color) string {
	if width <= 0 {
		return ""
	}
	base := lipgloss.NewStyle().Background(background).Foreground(chromeText)
	if right != "" && ansi.StringWidth(left)+ansi.StringWidth(right)+2 <= width {
		left += base.Render(strings.Repeat(" ", width-ansi.StringWidth(left)-ansi.StringWidth(right))) + right
	}
	return base.Render(fit(left, width))
}

func (m *model) titleBar() string {
	badge := lipgloss.NewStyle().Background(chromeAccent).Foreground(chromeBG).Bold(true).Render(" LYRA ")
	file := cleanLabel(filepath.Base(m.file))
	if file == "." || file == "" {
		file = "untitled.ly"
	}
	if m.untitled {
		file += " · sketch"
	}
	name := lipgloss.NewStyle().Background(chromeBG).Foreground(chromeText).Bold(true).Render("  " + file)
	right := ""
	if m.layout.width >= 88 {
		summary := m.audioSummary()
		if summary != "" {
			color := chromeMuted
			if m.audioState.Status == "Playing" || m.audioState.Status == "Ready" {
				color = chromeFocus
			}
			if m.audioState.Error != "" || m.audioErr != "" {
				color = lipgloss.Color("#e6a0aa")
			}
			right = lipgloss.NewStyle().Background(chromeBG).Foreground(color).Render(summary + " ")
		}
	}
	return bar(badge+name, right, m.layout.width, chromeBG)
}

func paneBar(label, detail string, width int, active bool) string {
	background, foreground, indicator := paneBG, chromeMuted, "○"
	if active {
		background, foreground, indicator = activePaneBG, chromeFocus, "●"
	}
	base := lipgloss.NewStyle().Background(background)
	left := base.Foreground(foreground).Bold(true).Render(" " + indicator + " " + label)
	right := ""
	if detail != "" {
		right = base.Foreground(chromeMuted).Render(detail + " ")
	}
	return bar(left, right, width, background)
}

func (m *model) paneBars() string {
	rightTitle, rightDetail := "SCORE", "PDF · "+strings.ToUpper(m.previewState.Theme.Label())
	if m.audioPane {
		rightTitle, rightDetail = "AUDIO", "TRANSPORT"
	}
	if m.diagnostics {
		rightTitle, rightDetail = "BUILDS", "DIAGNOSTICS"
	}
	if m.help {
		rightTitle, rightDetail = "HELP", "KEYS"
	}
	l := m.layout
	if l.previewCols > 0 {
		return paneBar("VIM", "LILYPOND", l.editorCols, m.focus == input.Editor) +
			dividerStyle.Background(paneBG).Render("│") +
			paneBar(rightTitle, rightDetail, l.previewCols, m.focus == input.Preview)
	}
	if m.focus == input.Editor {
		return paneBar("VIM", "LILYPOND", l.width, true)
	}
	return paneBar(rightTitle, rightDetail, l.width, true)
}
