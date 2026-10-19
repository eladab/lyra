//go:build darwin || linux

package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *launcher) listRows() int { return max(1, min(12, m.height-15)) }

func (m *launcher) templateRows() int { return max(1, min(10, m.height-16)) }

func (m *launcher) recentRows() int { return max(1, min(6, (m.height-12)/2)) }

func launchText(text string, width int) string {
	return fit(ansi.Truncate(cleanLabel(text), max(1, width), "…"), max(1, width))
}

func (m *launcher) fieldLine(width int) string {
	start := 0
	for start < m.cursor && ansi.StringWidth(cleanLabel(string(m.field[start:m.cursor]))) >= width {
		start++
	}
	prefix := cleanLabel(string(m.field[start:m.cursor]))
	character, suffix := " ", ""
	if m.cursor < len(m.field) {
		character = cleanLabel(string(m.field[m.cursor]))
		suffix = cleanLabel(string(m.field[m.cursor+1:]))
	}
	base := lipgloss.NewStyle().Background(paneBG).Foreground(chromeText)
	caret := lipgloss.NewStyle().Background(chromeFocus).Foreground(chromeBG)
	line := ansi.Truncate(base.Render(prefix)+caret.Render(character)+base.Render(suffix), width, "")
	return line + base.Render(strings.Repeat(" ", max(0, width-ansi.StringWidth(line))))
}

func (m *launcher) View() tea.View {
	w, h := max(1, m.width), max(1, m.height)
	background := lipgloss.NewStyle().Background(lipgloss.Color("#171d24")).Foreground(chromeText)
	if w < 28 || h < 18 {
		v := tea.NewView(background.Render(launchText("Lyra · enlarge terminal · Esc quits", w)))
		v.AltScreen = true
		return v
	}
	cardWidth := min(78, w-4)
	inner := cardWidth - 6
	text := lipgloss.NewStyle().Background(chromeBG).Foreground(chromeText)
	dim := text.Foreground(chromeMuted)
	accent := text.Foreground(chromeAccent).Bold(true)
	var rows []string
	rows = append(rows, accent.Render(launchText("LYRA", inner)), "")
	status, footer := m.notice, "↑/↓ select · Enter · o open · r recent · n new · Esc quit"
	switch m.page {
	case launchMenu:
		rows = append(rows, text.Bold(true).Render(launchText("Begin a composition", inner)), dim.Render(launchText("Open your music, or give a new score a home.", inner)), "")
		for i, option := range []struct{ title, detail string }{
			{"Open a score", "Browse folders and existing .ly files"},
			{"Open Recent", "Return to a recently opened score"},
			{"Create a score", "Choose a folder and name for a new composition"},
		} {
			style, marker := text, "  "
			if m.menu == i {
				style, marker = text.Background(activePaneBG).Foreground(chromeFocus).Bold(true), "› "
			}
			rows = append(rows, style.Render(launchText(marker+option.title, inner)), dim.Render(launchText("  "+option.detail, inner)))
			if h >= 21 {
				rows = append(rows, "")
			}
		}
	case launchRecent:
		footer = "↑/↓ move · Enter open · Esc back · q quit"
		rows = append(rows, text.Bold(true).Render(launchText("Open Recent", inner)), dim.Render(launchText("Scores opened in Lyra · newest first", inner)), "")
		count := m.recentRows()
		offset := max(0, min(m.selected-count/2, len(m.entries)-count))
		for row := range count {
			label, path, style := "", "", text
			index := offset + row
			if index < len(m.entries) {
				entry := m.entries[index]
				marker := "  "
				if index == m.selected {
					marker, style = "› ", text.Background(activePaneBG).Foreground(chromeFocus).Bold(true)
				}
				label, path = marker+entry.name, "  "+entry.path
			} else if row == 0 {
				label = "No recent scores yet"
				path = "Open or create a score to add it here."
			}
			if m.loading && row == 0 {
				label, path = "Reading recent scores…", ""
			}
			rows = append(rows, style.Render(launchText(label, inner)), dim.Render(launchText(path, inner)))
		}
		if !m.loading && len(m.entries) > 0 {
			status = fmt.Sprintf("%d/%d", m.selected+1, len(m.entries))
		}
	case launchBrowse:
		title := "Open a score"
		footer = "↑/↓ move · Enter open · ← parent · / path · n new · Esc back"
		if m.creating {
			title = "Choose a folder for your new score"
			footer = "Enter browse folder · n name score here · / path · Esc back"
		}
		rows = append(rows, text.Bold(true).Render(launchText(title, inner)), dim.Render(launchText(m.directory, inner)), "")
		count := m.listRows()
		offset := max(0, min(m.selected-count/2, len(m.entries)-count))
		for row := 0; row < count; row++ {
			index := offset + row
			label, style := "", text
			if index < len(m.entries) {
				entry := m.entries[index]
				prefix, suffix := "  ", ""
				if entry.dir {
					suffix = "/"
					style = style.Foreground(chromeFocus)
				}
				if index == m.selected {
					prefix = "› "
					style = style.Background(activePaneBG).Bold(true)
				}
				label = prefix + entry.name + suffix
			} else if row == 0 {
				label = "No .ly scores or subfolders here · n creates a score"
			}
			if m.loading && row == 0 {
				label = "Reading folder…"
			}
			rows = append(rows, style.Render(launchText(label, inner)))
		}
		if m.limited {
			status = "Folder listing limited; / enters a path directly"
		} else if !m.loading && len(m.entries) > 0 {
			status = fmt.Sprintf("%d/%d · . toggles hidden files · ~ home", m.selected+1, len(m.entries))
		}
	case launchCreate, launchPath:
		title, help := "Create a score", "Next, choose a template or start with a blank score."
		footer = "Enter continue · Tab complete · Ctrl+U clear · Esc back"
		if m.page == launchPath {
			title, help = "Open a path", "Enter a folder or an existing .ly score."
			footer = "Enter open · Tab complete · Ctrl+U clear · Esc back"
			if m.creating {
				title, help = "Go to a folder", "Browse to its location, then press n to name your score."
			}
		}
		rows = append(rows, text.Bold(true).Render(launchText(title, inner)), dim.Render(launchText(help, inner)), "", dim.Render(launchText("File location", inner)), m.fieldLine(inner), "", dim.Render(launchText("Relative paths start in: "+m.directory, inner)))
	case launchTemplate:
		footer = "↑/↓ select · Enter create · Ctrl+U clear · Esc filename"
		rows = append(rows, text.Bold(true).Render(launchText("Choose a template or start blank", inner)), dim.Render(launchText(m.destination, inner)), dim.Render(launchText("Search by topic, instrument or ensemble", inner)), m.fieldLine(inner), "")
		count := m.templateRows()
		offset := max(0, min(m.selected-count/2, len(m.templates)-count))
		for row := range count {
			label, style := "", text
			index := offset + row
			if index < len(m.templates) {
				marker := "  "
				if index == m.selected {
					marker, style = "› ", text.Background(activePaneBG).Foreground(chromeFocus).Bold(true)
				}
				label = marker + m.templates[index].Label()
			} else if row == 0 {
				label = "No matches · try fewer words or Ctrl+U for blank"
			}
			rows = append(rows, style.Render(launchText(label, inner)))
		}
		status = fmt.Sprintf("%d choices · LilyPond 2.26 · available offline", len(m.templates))
	}
	if m.busy {
		status = "Opening score…"
		if m.page == launchCreate {
			status = "Checking destination…"
		} else if m.page == launchTemplate {
			status = "Creating score…"
		}
	}
	style := dim
	if m.err != "" {
		status, style = m.err, text.Foreground(lipgloss.Color("#e6a0aa"))
	}
	rows = append(rows, "", style.Render(launchText(status, inner)), dim.Render(launchText(footer, inner)))
	// Fit every row before adding the frame. Lip Gloss's automatic wrapping would
	// otherwise split a long path or shortcut row inside the fixed-width dialog.
	border := text.Foreground(lipgloss.Color("#465260"))
	edge := border.Render("│")
	padding := text.Render("  ")
	blank := edge + text.Render(strings.Repeat(" ", inner+4)) + edge
	content := []string{border.Render("╭" + strings.Repeat("─", cardWidth-2) + "╮"), blank}
	for _, row := range rows {
		row = ansi.Truncate(row, inner, "")
		row += text.Render(strings.Repeat(" ", max(0, inner-ansi.StringWidth(row))))
		content = append(content, edge+padding+row+padding+edge)
	}
	content = append(content, blank, border.Render("╰"+strings.Repeat("─", cardWidth-2)+"╯"))
	left, top := max(0, (w-cardWidth)/2), max(0, (h-len(content))/2)
	frame := make([]string, h)
	for row := range frame {
		frame[row] = background.Render(strings.Repeat(" ", w))
		if row >= top && row-top < len(content) {
			frame[row] = background.Render(strings.Repeat(" ", left)) + content[row-top] + background.Render(strings.Repeat(" ", w-left-cardWidth))
		}
	}
	v := tea.NewView(strings.Join(frame, "\n"))
	v.AltScreen = true
	return v
}
