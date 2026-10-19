//go:build darwin || linux

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"lyra/internal/config"
	"lyra/internal/draft"
	"lyra/internal/scoretemplate"
)

type launchPage uint8

const (
	launchMenu launchPage = iota
	launchBrowse
	launchCreate
	launchPath
	launchTemplate
	launchRecent
)

const (
	launchOpenOption = iota
	launchRecentOption
	launchNewOption
	launchMenuOptions
)

type launchInput struct{ event uv.Event }
type launchEOF struct{}
type launchDirectory struct {
	version uint64
	entries []scoreEntry
	limited bool
	err     error
}
type launchChosen struct {
	version uint64
	path    string
	dir     bool
	err     error
}
type launchCompletion struct {
	version      uint64
	text, notice string
	err          error
}
type launchDestination struct {
	version uint64
	path    string
	err     error
}

type launcher struct {
	ctx                    context.Context
	events                 <-chan uv.Event
	page                   launchPage
	directory, suggested   string
	creating, hidden       bool
	menu, selected         int
	entries                []scoreEntry
	width, height          int
	field                  []rune
	cursor                 int
	err, notice            string
	loading, busy, limited bool
	version                uint64
	chosen                 string
	inputErr               error
	destination            string
	templates              []scoretemplate.Template
	recent                 config.RecentScores
}

func newLauncher(ctx context.Context, events <-chan uv.Event, directory string, width, height int) *launcher {
	name, err := draft.SuggestedName()
	if err != nil {
		name = "new-score.ly"
	}
	return &launcher{ctx: ctx, events: events, directory: directory, suggested: name, width: max(1, width), height: max(1, height), templates: scoretemplate.Search("")}
}

func (m *launcher) Init() tea.Cmd {
	return tea.Batch(m.readInput(), tea.Raw("\x1b[?2004h"))
}

func (m *launcher) readInput() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.events:
			if !ok {
				return launchEOF{}
			}
			return launchInput{event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *launcher) load(directory string) tea.Cmd {
	m.version++
	version, hidden := m.version, m.hidden
	m.page, m.directory = launchBrowse, directory
	m.entries, m.selected, m.loading = nil, 0, true
	m.err, m.notice = "", ""
	return func() tea.Msg {
		entries, limited, err := scoreDirectory(m.ctx, directory, hidden)
		return launchDirectory{version: version, entries: entries, limited: limited, err: err}
	}
}

func (m *launcher) pathField(page launchPage, value string) {
	m.version++ // Results for a previous folder/completion cannot change this form.
	m.page, m.field, m.cursor = page, []rune(value), utf8.RuneCountInString(value)
	m.err, m.notice, m.loading = "", "", false
}

func (m *launcher) newScore() {
	m.creating = true
	name := m.suggested
	existing := make(map[string]bool, len(m.entries))
	for _, entry := range m.entries {
		existing[entry.name] = true
	}
	for suffix := 2; ; suffix++ {
		if !existing[name] {
			break
		}
		name = fmt.Sprintf("%s-%d.ly", strings.TrimSuffix(m.suggested, ".ly"), suffix)
	}
	m.pathField(launchCreate, filepath.Join(m.directory, name))
}

func (m *launcher) choose(value string, create bool) tea.Cmd {
	m.version++
	version, directory := m.version, m.directory
	m.busy, m.err, m.notice = true, "", ""
	return func() tea.Msg {
		if create {
			path, err := scoreDestination(m.ctx, value, directory)
			return launchDestination{version: version, path: path, err: err}
		}
		path, dir, err := chooseScorePath(m.ctx, value, directory, false)
		return launchChosen{version: version, path: path, dir: dir, err: err}
	}
}

func (m *launcher) createTemplate() tea.Cmd {
	if len(m.templates) == 0 {
		return nil
	}
	m.version++
	version, path, content, ctx := m.version, m.destination, m.templates[m.selected].Code, m.ctx
	m.busy, m.err = true, ""
	return func() tea.Msg {
		chosen, err := createScore(ctx, path, "", content)
		return launchChosen{version: version, path: chosen, err: err}
	}
}

func (m *launcher) filterTemplates() {
	m.templates, m.selected = scoretemplate.Search(string(m.field)), 0
}

func (m *launcher) complete() tea.Cmd {
	m.version++
	version, directory, text := m.version, m.directory, string(m.field[:m.cursor])
	if m.cursor != len(m.field) {
		m.err = "Move to the end of the path to complete it"
		return nil
	}
	return func() tea.Msg {
		next, notice, err := completeScorePath(m.ctx, text, directory)
		return launchCompletion{version: version, text: next, notice: notice, err: err}
	}
}

func (m *launcher) insert(text string) {
	limit := 4096
	if m.page == launchTemplate {
		limit = 512
	}
	if len(string(m.field))+len(text) > limit || strings.IndexFunc(text, unicode.IsControl) >= 0 {
		m.err = "Enter or paste one path, without line breaks or control characters"
		if m.page == launchTemplate {
			m.err = "Search with up to 512 bytes, without line breaks or control characters"
		}
		return
	}
	runes := []rune(text)
	next := make([]rune, 0, len(m.field)+len(runes))
	next = append(next, m.field[:m.cursor]...)
	next = append(next, runes...)
	next = append(next, m.field[m.cursor:]...)
	m.field, m.cursor = next, m.cursor+len(runes)
	m.version++
	m.err, m.notice = "", ""
	if m.page == launchTemplate {
		m.filterTemplates()
	}
}

func (m *launcher) fieldKey(key uv.KeyPressEvent) tea.Cmd {
	query := string(m.field)
	defer func() {
		if m.page == launchTemplate && query != string(m.field) {
			m.filterTemplates()
		}
	}()
	if !key.MatchString("esc", "enter", "tab") {
		m.version++
		m.err, m.notice = "", ""
	}
	switch key.String() {
	case "esc":
		if m.page == launchTemplate {
			m.pathField(launchCreate, m.destination)
			return nil
		}
		return m.load(m.directory)
	case "enter":
		if m.page == launchTemplate {
			return m.createTemplate()
		}
		return m.choose(string(m.field), m.page == launchCreate)
	case "tab":
		if m.page == launchTemplate {
			m.selected = min(max(0, len(m.templates)-1), m.selected+1)
			return nil
		}
		return m.complete()
	case "left":
		m.cursor = max(0, m.cursor-1)
	case "right":
		m.cursor = min(len(m.field), m.cursor+1)
	case "home", "ctrl+a":
		m.cursor = 0
	case "end", "ctrl+e":
		m.cursor = len(m.field)
	case "ctrl+u":
		m.field, m.cursor = nil, 0
		m.version++
	case "ctrl+w":
		end := m.cursor
		for m.cursor > 0 && unicode.IsSpace(m.field[m.cursor-1]) {
			m.cursor--
		}
		for m.cursor > 0 && !unicode.IsSpace(m.field[m.cursor-1]) && m.field[m.cursor-1] != '/' {
			m.cursor--
		}
		if end == m.cursor && m.cursor > 0 {
			m.cursor--
		}
		m.field = append(m.field[:m.cursor], m.field[end:]...)
		m.version++
	case "backspace", "ctrl+h":
		if m.cursor > 0 {
			m.field = append(m.field[:m.cursor-1], m.field[m.cursor:]...)
			m.cursor--
			m.version++
		}
	case "delete":
		if m.cursor < len(m.field) {
			m.field = append(m.field[:m.cursor], m.field[m.cursor+1:]...)
			m.version++
		}
	case "up", "ctrl+p", "shift+tab":
		if m.page == launchTemplate {
			m.selected = max(0, m.selected-1)
		}
	case "down", "ctrl+n":
		if m.page == launchTemplate {
			m.selected = min(max(0, len(m.templates)-1), m.selected+1)
		}
	case "pgup", "pgdown":
		if m.page == launchTemplate {
			delta := m.templateRows()
			if key.String() == "pgup" {
				delta = -delta
			}
			m.selected = max(0, min(max(0, len(m.templates)-1), m.selected+delta))
		}
	default:
		if key.Mod&(uv.ModCtrl|uv.ModAlt|uv.ModMeta) == 0 {
			m.insert(key.Text)
		}
	}
	return nil
}

func (m *launcher) key(key uv.KeyPressEvent) tea.Cmd {
	if key.MatchString("ctrl+c") || ((m.width < 28 || m.height < 18) && key.MatchString("esc")) {
		return tea.Quit
	}
	if m.busy {
		return nil
	}
	if m.page == launchCreate || m.page == launchPath || m.page == launchTemplate {
		return m.fieldKey(key)
	}
	if m.page == launchRecent {
		return m.recentKey(key)
	}
	if m.page == launchMenu {
		switch key.String() {
		case "esc", "q":
			return tea.Quit
		case "up", "k", "shift+tab":
			m.menu = (m.menu + launchMenuOptions - 1) % launchMenuOptions
		case "down", "j", "tab":
			m.menu = (m.menu + 1) % launchMenuOptions
		case "o", "1":
			m.creating = false
			return m.load(m.directory)
		case "r", "2":
			return m.loadRecent()
		case "n", "3":
			m.creating = true
			return m.load(m.directory)
		case "enter":
			if m.menu == launchRecentOption {
				return m.loadRecent()
			}
			m.creating = m.menu == launchNewOption
			return m.load(m.directory)
		}
		return nil
	}
	switch key.String() {
	case "esc":
		m.version++
		m.page, m.loading, m.err = launchMenu, false, ""
	case "q":
		return tea.Quit
	case "up", "k":
		m.selected = max(0, m.selected-1)
	case "down", "j":
		m.selected = min(max(0, len(m.entries)-1), m.selected+1)
	case "pgup":
		m.selected = max(0, m.selected-m.listRows())
	case "pgdown":
		m.selected = min(max(0, len(m.entries)-1), m.selected+m.listRows())
	case "home", "g":
		m.selected = 0
	case "end", "G":
		m.selected = max(0, len(m.entries)-1)
	case "left", "backspace", "ctrl+h":
		return m.load(filepath.Dir(m.directory))
	case "~":
		home, err := scorePath("~", m.directory)
		if err == nil {
			return m.load(home)
		}
		m.err = err.Error()
	case ".":
		m.hidden = !m.hidden
		return m.load(m.directory)
	case "/":
		m.pathField(launchPath, m.directory+string(filepath.Separator))
	case "n":
		m.newScore()
	case "enter", "right":
		if !m.loading && len(m.entries) > 0 {
			entry := m.entries[m.selected]
			if entry.dir {
				return m.load(entry.path)
			}
			if m.creating {
				m.err = "That score already exists; press n to choose a new name here"
			} else {
				return m.choose(entry.path, false)
			}
		}
	}
	return nil
}

func (m *launcher) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case launchInput:
		var cmd tea.Cmd
		switch event := msg.event.(type) {
		case uv.KeyPressEvent:
			cmd = m.key(event)
		case uv.PasteEvent:
			if !m.busy && (m.page == launchCreate || m.page == launchPath || m.page == launchTemplate) {
				m.insert(event.Content)
			}
		}
		return m, tea.Batch(cmd, m.readInput())
	case launchEOF:
		m.inputErr = fmt.Errorf("terminal input closed in startup chooser")
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case launchDirectory:
		if msg.version == m.version {
			m.entries, m.limited, m.loading = msg.entries, msg.limited, false
			if msg.err != nil {
				m.err = "Read folder: " + msg.err.Error()
			}
		}
	case launchRecents:
		m.setRecent(msg)
	case launchChosen:
		if msg.version == m.version {
			m.busy = false
			if msg.err != nil {
				m.err = msg.err.Error()
			} else if msg.dir && m.page == launchRecent {
				m.err = "That recent score is now a folder; choose another score"
			} else if msg.dir {
				return m, m.load(msg.path)
			} else if m.creating && m.page == launchPath {
				m.err = "That file already exists; browse to its folder and press n to create a score"
			} else {
				m.chosen = msg.path
				return m, tea.Quit
			}
		}
	case launchDestination:
		if msg.version == m.version {
			m.busy = false
			if msg.err != nil {
				m.err = msg.err.Error()
			} else {
				m.destination = msg.path
				m.pathField(launchTemplate, "")
				m.filterTemplates()
			}
		}
	case launchCompletion:
		if msg.version == m.version {
			m.notice = msg.notice
			if msg.err != nil {
				m.err = msg.err.Error()
			} else {
				m.field, m.cursor = []rune(msg.text), utf8.RuneCountInString(msg.text)
			}
		}
	}
	return m, nil
}
