//go:build darwin || linux

package app

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"lyra/internal/audio"
	"lyra/internal/editor"
	"lyra/internal/input"
	"lyra/internal/lilypond"
	"lyra/internal/preview"
)

type rawMsg struct {
	data []byte
	err  error
}
type changedMsg struct{}
type exitedMsg struct{ err error }
type flushMsg uint64
type buildMsg lilypond.State

type model struct {
	session                   *editor.Session
	reader                    io.Reader
	router                    input.Router
	layout                    layout
	file                      string
	focus                     input.Action
	help                      bool
	generation                uint64
	err                       error
	builds                    *lilypond.Service
	buildState                lilypond.State
	buildErr                  string
	paneScroll                int
	showLog                   bool
	diagnostics               bool
	fullPage                  bool
	split, zoom               float64
	page, scrollX, scrollY    int
	cellWidth, cellHeight     int
	imageID                   uint32
	graphics, mouse, dragging bool
	graphicsErr, previewErr   string
	previews                  *preview.Service
	previewState              preview.State
	previewTheme              preview.Theme
	previewVersion            uint64
	previewPending            bool
	frames                    *frameRenderer
	framePending, dirty       bool
	redraw                    bool
	audio                     *audio.Service
	audioState                audio.State
	audioErr                  string
	audioPane                 bool
	docs                      docsPane
	pdfViewer                 *pdfViewer
	pdfViewerState            pdfViewerState
	pdfViewerNotice           string
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.readInput(), m.waitEditor(), m.waitBuild(), m.waitPreview(), m.waitAudio(), m.waitDocsLookup(), m.waitPDFViewer(), m.scheduleFrame(), probeTimer())
}

func (m *model) readInput() tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := m.reader.Read(buf)
		return rawMsg{data: buf[:n], err: err}
	}
}

func (m *model) waitEditor() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.session.Done():
			return exitedMsg{m.session.Err()}
		case <-m.session.Changed():
			return changedMsg{}
		}
	}
}

func (m *model) dispatch(events []input.Event) error {
	for _, event := range events {
		if event.Action != input.HostReply && event.Action != input.MouseEvent {
			m.pdfViewerNotice = ""
		}
		switch event.Action {
		case input.OpenPDF:
			m.openPDFViewer()
		case input.Documentation:
			m.openDocs("", false)
		case input.Audio:
			m.docs.visible = false
			m.focus, m.help, m.diagnostics, m.audioPane, m.paneScroll = input.Preview, false, false, true, 0
		case input.PlayPause:
			m.audioAction(audio.Toggle)
		case input.StopAudio:
			m.audioAction(audio.Stop)
		case input.SeekBack:
			m.audioAction(audio.Backward)
		case input.SeekForward:
			m.audioAction(audio.Forward)
		case input.LatestAudio:
			m.audioAction(audio.Latest)
		case input.PreviewTheme:
			m.cyclePreviewTheme()
		case input.HostReply:
			m.hostReply(event)
		case input.MouseEvent:
			if err := m.mouseEvent(event); err != nil {
				return err
			}
		case input.Narrower:
			if err := m.resizeSplit(m.split - .05); err != nil {
				return err
			}
		case input.Wider:
			if err := m.resizeSplit(m.split + .05); err != nil {
				return err
			}
		case input.EqualSplit:
			if err := m.resizeSplit(.5); err != nil {
				return err
			}
		case input.Diagnostics:
			m.docs.visible = false
			m.focus, m.help, m.diagnostics, m.paneScroll = input.Preview, false, true, 0
			m.audioPane = false
		case input.ToggleMouse:
			m.mouse = !m.mouse
			m.dragging = false
		case input.Editor:
			m.focus, m.help = input.Editor, false
		case input.Preview:
			m.docs.visible = false
			m.focus, m.help = input.Preview, false
			m.diagnostics = false
			m.audioPane = false
		case input.Help:
			m.docs.visible = false
			m.focus, m.help, m.paneScroll = input.Preview, true, 0
			m.audioPane = false
		case input.Build:
			if m.builds != nil {
				m.builds.Request(true)
			}
		case input.CancelBuild:
			if m.builds != nil {
				m.builds.Cancel()
			}
		case input.Quit:
			m.focus, m.help = input.Editor, false
			if err := m.session.RequestQuit(); err != nil {
				return err
			}
		default:
			if m.focus == input.Editor {
				if err := m.session.WriteInput(event.Data); err != nil {
					return err
				}
			} else if m.docs.visible {
				m.docsKeys(event.Data)
			} else if string(event.Data) == "q" || string(event.Data) == "\x1b" {
				m.focus, m.help = input.Editor, false
			} else if !m.help && m.audioPane {
				m.audioKeys(event.Data)
			} else if !m.help && !m.diagnostics && m.frames != nil {
				m.previewKeys(event.Data)
			} else if !m.help {
				for _, key := range event.Data {
					switch key {
					case 'j':
						m.paneScroll++
					case 'k':
						m.paneScroll = max(0, m.paneScroll-1)
					case 'g':
						m.paneScroll = 0
					case 'G':
						m.paneScroll = max(0, len(m.buildLines(m.paneWidth()))-m.layout.editorRows)
					case 'l':
						m.showLog, m.paneScroll = !m.showLog, 0
					case 'd', 'p':
						m.diagnostics = false
					}
				}
				m.paneScroll = min(m.paneScroll, max(0, len(m.buildLines(m.paneWidth()))-m.layout.editorRows))
			}
		}
	}
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameTick:
		m.dirty = false
		return m, tea.Raw(frameOutput(m.frames.frame(m)))
	case tea.RawMsg:
		if _, ok := msg.(tea.RawMsg).Msg.(frameOutput); ok {
			m.framePending = false
			if m.dirty {
				return m, m.scheduleFrame()
			}
			return m, nil
		}
	}
	_, cmd := m.update(msg)
	return m, tea.Batch(cmd, m.loadDocs(), m.scheduleFrame())
}

func (m *model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pdfViewerMsg:
		m.pdfViewerState = pdfViewerState(msg)
		switch {
		case msg.Err != nil:
			m.pdfViewerNotice = "Open PDF: " + cleanLabel(msg.Err.Error()) + " · Ctrl+\\ d for details"
		case msg.Busy:
			m.pdfViewerNotice = "Opening PDF in default viewer..."
		default:
			m.pdfViewerNotice = "PDF sent to default viewer · save a copy before closing Lyra"
		}
		return m, m.waitPDFViewer()
	case docsLookup:
		m.docs.lastRequest = msg.id
		m.openDocs(msg.query, true)
		return m, m.waitDocsLookup()
	case docsLoaded:
		m.docs.loading = false
		if msg.catalog != nil {
			m.docs.catalog = msg.catalog
			m.docs.section = -1
			m.docs.searching = true
			m.docs.scroll = 0
			m.docs.search()
		}
		if msg.err != nil {
			m.docs.err = msg.err.Error()
		}
	case probeTimeout:
		if !m.graphics {
			m.graphicsErr = "Terminal has no Kitty graphics support. Use Ghostty or Kitty."
		} else if m.cellWidth < 1 {
			m.graphicsErr = "Terminal did not report pixel dimensions; PDF preview unavailable."
		}
	case previewMsg:
		if msg.Version == m.previewVersion {
			if msg.Err == nil {
				m.previewState = preview.State(msg)
			} else {
				m.previewState.Err = msg.Err
			}
			m.previewPending = false
			if msg.Err == nil {
				m.page, m.scrollX, m.scrollY = msg.Page, msg.ScrollX, msg.ScrollY
			}
		}
		return m, m.waitPreview()
	case audioMsg:
		m.audioState = audio.State(msg)
		return m, m.waitAudio()
	case buildMsg:
		if msg.Generation != m.buildState.Generation {
			m.paneScroll = 0
		}
		old := m.buildState.LastGood
		m.buildState = lilypond.State(msg)
		if msg.LastGood != nil && (old == nil || old.Generation != msg.LastGood.Generation) {
			m.updateAudioSource()
			if len(msg.LastGood.PDFs) == 0 {
				m.previewVersion++
				m.previewState = preview.State{}
				m.previewPending = false
				if m.previews != nil {
					m.previews.Cancel()
				}
			} else {
				m.requestPreview()
			}
		}
		return m, m.waitBuild()
	case tea.WindowSizeMsg:
		next := measureSplit(msg.Width, msg.Height, m.split)
		if next.editorCols != m.layout.editorCols || next.editorRows != m.layout.editorRows {
			if err := m.session.Resize(next.editorCols, next.editorRows); err != nil {
				select {
				case <-m.session.Done():
				default:
					m.err = fmt.Errorf("resize editor: %w", err)
				}
				return m, tea.Quit
			}
		}
		m.layout = next
		m.requestPreview()
		if m.frames != nil {
			return m, tea.Raw("\x1b[16t\x1b[14t")
		}
	case rawMsg:
		if err := m.dispatch(m.router.Feed(msg.data)); err != nil {
			m.err = fmt.Errorf("send input to Vim: %w", err)
			return m, tea.Quit
		}
		if msg.err != nil {
			m.err = fmt.Errorf("terminal input closed: %w", msg.err)
			return m, tea.Quit
		}
		m.generation++
		generation := m.generation
		return m, tea.Batch(m.readInput(), tea.Tick(25*time.Millisecond, func(time.Time) tea.Msg { return flushMsg(generation) }))
	case flushMsg:
		if uint64(msg) == m.generation {
			if err := m.dispatch(m.router.Flush()); err != nil {
				m.err = err
				return m, tea.Quit
			}
		}
	case changedMsg:
		return m, m.waitEditor()
	case exitedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("Vim exited: %w", msg.err)
		}
		return m, tea.Quit
	}
	return m, nil
}

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#bcacff")).Bold(true)
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8792a2"))
)

// fit keeps each pane in its own cells. Only emulator-generated styling enters
// the editor view; child cursor movements never reach the host terminal.
func fit(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	return s + strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
}

func cleanLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func (m *model) preview(width, rows int) []string {
	source := m.buildLines(width)
	if m.frames != nil && !m.diagnostics {
		source = []string{m.previewStatus()}
		if m.previewState.Payload == "" {
			source = append(source, "", "Ctrl+\\ d: build diagnostics", "n/p pages · +/- zoom · 0 fit", "f content/page · hjkl scroll", "t paper theme · r reload", "Ctrl+\\ o: open PDF externally", "Ctrl+\\ [ / ]: adjust split", "Ctrl+\\ m: toggle mouse")
		}
	}
	if m.help {
		source = strings.Split("Lyra keys\nCtrl+\\ then:\n e Vim · p PDF · a audio\n d diagnostics · h help · q quit\n Space play/pause · s stop\n ,/. seek 5s · u latest take\n [/] split · = half · m mouse\n t preview paper theme\n b rebuild · c cancel build\n Ctrl+\\ send literal prefix\n\nPDF: n/p pages · +/- zoom\n 0 reset fit · f content/page\n t warm/night/original paper\n hjkl/arrows scroll · r redraw\n Space play/pause · s stop\n ,/. seek · u latest take\n\nAudio: Space play/pause\n s stop · 0 restart · +/- volume\n ,/. or arrows seek 5s\n u play latest from start\n\nBuilds: j/k scroll · l raw log\n g/G first/last · d/p return PDF\nq/Esc in a pane returns to Vim\nVim: :w, :q, :wq as usual", "\n")
		source = append(source[:4], append([]string{" D docs search · Vim K lookup"}, source[4:]...)...)
		source = append(source[:5], append([]string{" o open PDF in default viewer"}, source[5:]...)...)
	}
	if m.audioPane && !m.help {
		source = m.audioLines(width)
	}
	if m.docs.visible {
		source = m.docsLines(width, rows)
	}
	offset := min(m.paneScroll, max(0, len(source)-rows))
	if m.docs.visible {
		offset = 0
	}
	lines := make([]string, rows)
	for y := range rows {
		if y+offset < len(source) {
			if y == 0 && offset == 0 && m.frames != nil && !m.help && !m.diagnostics && !m.audioPane && !m.docs.visible {
				lines[y] = lipgloss.NewStyle().Foreground(chromeMuted).Background(chromeBG).Render(fit(source[0], width))
			} else {
				lines[y] = muted.Render(fit(source[y+offset], width))
			}
		} else {
			lines[y] = strings.Repeat(" ", width)
		}
	}
	return lines
}

func (m *model) View() tea.View {
	l := m.layout
	snap := m.session.Terminal.Snapshot()
	editorLines := strings.Split(snap.Content, "\n")
	previewWidth := l.previewCols
	if previewWidth == 0 {
		previewWidth = l.width
	}
	previewLines := m.preview(previewWidth, l.editorRows)
	lines := []string{m.titleBar(), m.paneBars()}
	for y := range l.editorRows {
		line := ""
		if y < len(editorLines) {
			line = editorLines[y]
		}
		if l.previewCols > 0 {
			lines = append(lines, fit(line, l.editorCols)+dividerStyle.Render("│")+previewLines[y])
		} else if m.focus == input.Editor {
			lines = append(lines, fit(line, l.width))
		} else {
			lines = append(lines, previewLines[y])
		}
	}
	footer := " Ctrl+\\: Space play · s stop · a audio · e Vim · p PDF · o open PDF · D docs · t theme · [/] split · h help · q quit"
	if m.pdfViewerNotice != "" {
		footer = " " + m.pdfViewerNotice
	}
	if m.mouse {
		footer += " · MOUSE ON"
	}
	lines = append(lines, muted.Render(fit(footer, l.width)))
	if l.height < 5 {
		lines = []string{fit("Lyra: enlarge terminal", l.width)}
	}
	v := tea.NewView(strings.Join(lines[:min(len(lines), l.height)], "\n"))
	v.AltScreen = true
	if m.focus == input.Editor && !snap.Hidden && l.height >= 5 && snap.X < l.editorCols && snap.Y < l.editorRows {
		v.Cursor = tea.NewCursor(snap.X, snap.Y+2)
		v.Cursor.Shape, v.Cursor.Blink = tea.CursorShape(snap.Style), snap.Blink
	}
	return v
}
