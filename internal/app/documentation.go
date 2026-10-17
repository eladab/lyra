//go:build darwin || linux

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	documentation "lyra/internal/docs"
	"lyra/internal/input"
)

type docsPane struct {
	ctx          context.Context
	loader       documentation.Loader
	catalog      *documentation.Catalog
	notification string
	lastRequest  string
	visible      bool
	searching    bool
	query        string
	results      []int
	selected     int
	section      int
	scroll       int
	wantLoad     bool
	refresh      bool
	loading      bool
	lookup       bool
	err          string
	keys         []byte
	pasting      bool
}

type docsLoaded struct {
	catalog *documentation.Catalog
	err     error
}
type docsLookup struct{ id, query string }

func (m *model) waitDocsLookup() tea.Cmd {
	d := &m.docs
	if d.notification == "" || d.ctx == nil {
		return nil
	}
	ctx, path, last := d.ctx, d.notification, d.lastRequest
	return func() tea.Msg {
		timer := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
				f, err := os.Open(path)
				if err != nil {
					continue
				}
				data, err := io.ReadAll(io.LimitReader(f, 4097))
				_ = f.Close()
				var request []string
				if err == nil && len(data) <= 4096 && json.Unmarshal(data, &request) == nil && len(request) == 2 && request[0] != last {
					return docsLookup{request[0], request[1]}
				}
			}
		}
	}
}

func (m *model) loadDocs() tea.Cmd {
	d := &m.docs
	if !d.wantLoad || d.loading {
		return nil
	}
	d.wantLoad, d.loading, d.err = false, true, ""
	loader, refresh, ctx := d.loader, d.refresh, d.ctx
	d.refresh = false
	if ctx == nil {
		ctx = context.Background()
	}
	return func() tea.Msg {
		// Bound both downloads together, including any connection stalls.
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		catalog, err := loader.Load(ctx, refresh)
		return docsLoaded{catalog, err}
	}
}

func (m *model) openDocs(query string, lookup bool) {
	d := &m.docs
	d.visible, d.searching, d.lookup = true, true, lookup
	d.query = cleanLabel(query)
	if len(d.query) > 512 {
		d.query = string([]rune(d.query)[:min(128, len([]rune(d.query)))])
	}
	d.keys, d.pasting = nil, false
	d.selected, d.scroll = 0, 0
	m.focus, m.help, m.diagnostics, m.audioPane = input.Preview, false, false, false
	if d.catalog == nil && !d.loading {
		d.wantLoad = true
	}
	d.search()
}

func (d *docsPane) search() {
	if d.catalog == nil {
		return
	}
	d.results = d.catalog.Search(d.query, 200)
	d.selected = min(d.selected, max(0, len(d.results)-1))
	if d.lookup && len(d.results) > 0 {
		d.section, d.searching = d.results[0], false
	}
	d.lookup = false
}

// docsKeys decodes only pane input. Editor bytes still pass straight to Vim.
// Partial UTF-8, navigation keys and paste delimiters survive raw-read splits.
func (m *model) docsKeys(data []byte) {
	d := &m.docs
	query := d.query
	search := func() {
		if query != d.query {
			d.lookup = false
			d.search()
			query = d.query
		}
	}
	defer search()
	if len(d.keys)+len(data) > 8192 {
		d.keys = nil
		return
	}
	d.keys = append(d.keys, data...)
	for len(d.keys) > 0 {
		marker := []byte("\x1b[200~")
		if d.pasting {
			marker = []byte("\x1b[201~")
		}
		if bytes.HasPrefix(marker, d.keys) && len(d.keys) < len(marker) && len(d.keys) > 1 {
			return
		}
		if bytes.HasPrefix(d.keys, marker) {
			d.pasting = !d.pasting
			d.keys = d.keys[len(marker):]
			continue
		}
		if !d.pasting && d.keys[0] == 0x1b {
			if len(d.keys) == 1 {
				d.keys = nil
				m.focus = input.Editor
				return
			}
			if d.keys[1] == '[' || d.keys[1] == 'O' {
				end := 2
				for end < len(d.keys) && (d.keys[end] < 0x40 || d.keys[end] > 0x7e) {
					end++
				}
				if end == len(d.keys) {
					return
				}
				key := string(d.keys[:end+1])
				d.keys = d.keys[end+1:]
				search()
				switch key {
				case "\x1b[A", "\x1bOA":
					d.move(-1)
				case "\x1b[B", "\x1bOB":
					d.move(1)
				case "\x1b[5~":
					d.move(-max(1, m.layout.editorRows-4))
				case "\x1b[6~":
					d.move(max(1, m.layout.editorRows-4))
				}
				continue
			}
			// An unrecognized escape must never become a documentation command.
			d.keys = nil
			return
		}
		if !utf8.FullRune(d.keys) {
			return
		}
		r, n := utf8.DecodeRune(d.keys)
		d.keys = d.keys[n:]
		if d.pasting {
			if d.searching {
				if r == '\n' || r == '\r' || r == '\t' {
					r = ' '
				}
				d.appendQuery(r)
			}
			continue
		}
		if r == 0x12 || r == 'R' && !d.searching {
			d.wantLoad, d.refresh = !d.loading, true
			continue
		}
		if d.searching {
			switch r {
			case '\r', '\n':
				search()
				if d.catalog == nil && !d.loading {
					d.wantLoad = true
				}
				if len(d.results) > 0 {
					d.section, d.searching, d.scroll = d.results[d.selected], false, 0
				}
			case 0x7f, 0x08:
				_, n := utf8.DecodeLastRuneInString(d.query)
				d.query = d.query[:len(d.query)-n]
				d.selected = 0
			case 0x15:
				d.query, d.selected = "", 0
			case 0x0e, '\t':
				search()
				d.move(1)
			case 0x10:
				search()
				d.move(-1)
			default:
				d.appendQuery(r)
			}
		} else {
			switch r {
			case '/', 'b':
				d.searching = true
			case 'q':
				m.focus = input.Editor
			case 'j':
				d.move(1)
			case 'k':
				d.move(-1)
			case ' ', 0x06:
				d.move(max(1, m.layout.editorRows-4))
			case 0x02:
				d.move(-max(1, m.layout.editorRows-4))
			case 'g':
				d.scroll = 0
			case 'G':
				d.scroll = max(0, len(m.docsArticle(m.paneWidth()))-max(1, m.layout.editorRows-2))
			}
		}
	}
}

func (d *docsPane) appendQuery(r rune) {
	if unicode.IsPrint(r) && len(d.query)+utf8.RuneLen(r) <= 512 {
		d.query += string(r)
		d.selected = 0
	}
}

func (d *docsPane) move(delta int) {
	if d.searching {
		d.selected = max(0, min(max(0, len(d.results)-1), d.selected+delta))
	} else {
		d.scroll = max(0, d.scroll+delta)
	}
}

func wrapDocs(text string, width int) []string {
	var rows []string
	for _, line := range strings.Split(text, "\n") {
		rows = append(rows, strings.Split(ansi.Hardwrap(cleanLabel(strings.ReplaceAll(line, "\t", "    ")), max(1, width), true), "\n")...)
	}
	return rows
}

func (m *model) docsArticle(width int) []string {
	d := &m.docs
	if d.catalog == nil || d.section < 0 || d.section >= len(d.catalog.Sections) {
		return nil
	}
	s := d.catalog.Sections[d.section]
	return wrapDocs(s.Title+"\n"+s.Manual+" · LilyPond "+d.catalog.Version+"\n\n"+s.Text+"\n\nSource: "+s.URL+"\nLilyPond development team · GNU FDL\nLicense: https://lilypond.org/doc/v"+d.catalog.Version+"/Documentation/notation/gnu-free-documentation-license.html", width)
}

func (m *model) docsLines(width, rows int) []string {
	d := &m.docs
	var lines []string
	if d.searching {
		lines = []string{"Search: " + d.query + "▏", "↑/↓ select · Enter read · Esc Vim", "Ctrl+R refresh cached manuals"}
	} else {
		lines = []string{"/ search · b results · R refresh", "j/k scroll · Space page · q/Esc Vim"}
	}
	if d.loading {
		lines = append(lines, "Loading/indexing LilyPond manuals…")
	}
	if d.err != "" {
		lines = append(lines, wrapDocs(d.err, width)...)
		if d.catalog == nil {
			lines = append(lines, "Ctrl+\\ D then Enter to retry.")
		}
	}
	if d.catalog == nil {
		return lines
	}
	if d.searching {
		available := max(1, rows-len(lines)-1)
		lines = append(lines, fmt.Sprintf("%d topics · LilyPond %s · local index", len(d.results), d.catalog.Version))
		start := max(0, d.selected-available+1)
		if len(d.results) == 0 {
			lines = append(lines, "No topics match. Try a command or term.")
		}
		for i := start; i < min(len(d.results), start+available); i++ {
			s := d.catalog.Sections[d.results[i]]
			marker := "  "
			if i == d.selected {
				marker = "› "
			}
			lines = append(lines, marker+s.Title+" · "+s.Manual)
		}
	} else {
		article := m.docsArticle(width)
		d.scroll = min(d.scroll, max(0, len(article)-max(1, rows-len(lines))))
		lines = append(lines, article[d.scroll:]...)
	}
	return lines
}
