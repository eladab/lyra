//go:build darwin || linux

package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"lyra/internal/docs"
	"lyra/internal/input"
)

func docsCatalog() *docs.Catalog {
	return &docs.Catalog{Version: "2.26", Sections: []docs.Section{
		{Title: "Tuplets", Manual: "Notation Reference", URL: "https://lilypond.org/tuplet", Terms: []string{`\tuplet`}, Text: "Group notes in tuplets.\n    \\tuplet 3/2 { c8 d e }"},
		{Title: "Hello lookup", Manual: "Learning Manual", URL: "https://lilypond.org/hello", Terms: []string{"hello"}, Text: "Lookup topic displayed inside Lyra."},
	}}
}

func TestDocsLookupSearchAndRetainedPDF(t *testing.T) {
	m := renderModel(t)
	m.docs.catalog = docsCatalog()
	l := m.layout
	m.frames.frame(m)
	m.update(docsLookup{"request-1", `\tuplet`})
	if m.docs.section != 0 || m.docs.searching || !m.docs.visible || m.focus != input.Preview || m.layout != l {
		t.Fatal("keyword lookup failed or changed editor geometry")
	}
	if frame := m.frames.frame(m); !strings.Contains(frame, "a=d,d=I,i=12345") || strings.Contains(frame, "a=T") {
		t.Fatal("documentation did not clear the PDF image")
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "DOCS") || !strings.Contains(view, "Group notes in tuplets") {
		t.Fatal("documentation was not rendered in its pane", view)
	}
	m.docsKeys([]byte("/\x15hlolkp\r"))
	if m.docs.section != 1 || m.docs.searching {
		t.Fatal("fuzzy search did not select another topic", m.docs)
	}
	m.docsKeys([]byte("q"))
	if m.focus != input.Editor || !m.docs.visible || m.previewState.Payload != "AA==" || m.layout != l {
		t.Fatal("return to editor lost pane or score state")
	}
	if err := m.dispatch([]input.Event{{Action: input.Preview}}); err != nil {
		t.Fatal(err)
	}
	if m.docs.visible || !strings.Contains(m.frames.frame(m), "a=T") {
		t.Fatal("PDF did not return after documentation")
	}
}

func TestDocsInputChunkBoundariesPasteAndUnicode(t *testing.T) {
	data := []byte("\x1cD\x1b[200~q\n\x1cqé♩\x1b[201~")
	for split := range len(data) + 1 {
		m := &model{focus: input.Editor, docs: docsPane{catalog: docsCatalog()}}
		if err := m.dispatch(m.router.Feed(data[:split])); err != nil {
			t.Fatal(err)
		}
		if err := m.dispatch(m.router.Feed(data[split:])); err != nil {
			t.Fatal(err)
		}
		if m.focus != input.Preview || m.docs.query != "q qé♩" || m.docs.pasting {
			t.Fatalf("split=%d focus=%d query=%q paste=%t", split, m.focus, m.docs.query, m.docs.pasting)
		}
		m.docsKeys([]byte{0x7f})
		if m.docs.query != "q qé" {
			t.Fatal("backspace split UTF-8", m.docs.query)
		}
	}
	// Each character of a real navigation key can arrive in a different read.
	m := &model{focus: input.Preview, docs: docsPane{catalog: docsCatalog(), visible: true, searching: true}}
	m.docs.search()
	for _, b := range []byte("\x1b[B") {
		if err := m.dispatch(m.router.Feed([]byte{b})); err != nil {
			t.Fatal(err)
		}
	}
	if m.docs.selected != 1 || m.focus != input.Preview {
		t.Fatal("split arrow was treated as text or Esc")
	}
}

func TestDocsLoadFailureAndRefreshRetainGoodCatalog(t *testing.T) {
	m := &model{docs: docsPane{catalog: docsCatalog(), section: 0, loading: true}}
	m.update(docsLoaded{err: errors.New("network unavailable")})
	if m.docs.catalog == nil || m.docs.section != 0 || m.docs.err != "network unavailable" || m.docs.loading {
		t.Fatal("refresh failure discarded usable documentation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.docs = docsPane{ctx: ctx, notification: filepath.Join(t.TempDir(), "lookup.json")}
	if err := os.WriteFile(m.docs.notification, []byte(`["first","tuplet"]`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := m.waitDocsLookup()
	msg := cmd().(docsLookup)
	if msg.query != "tuplet" || msg.id != "first" {
		t.Fatal(msg)
	}
	m.docs.lastRequest = msg.id
	cmd = m.waitDocsLookup()
	cancel()
	done := make(chan struct{})
	go func() { cmd(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("documentation notification reader ignored cancellation")
	}
}

func TestDocsPaneFitsEveryTerminalSize(t *testing.T) {
	m := renderModel(t)
	m.docs.catalog = docsCatalog()
	m.openDocs("", false)
	for _, size := range [][2]int{{1, 1}, {20, 5}, {72, 18}, {88, 24}, {120, 32}} {
		m.layout = measure(size[0], size[1])
		for _, search := range []bool{true, false} {
			m.docs.searching, m.docs.section = search, 0
			for _, row := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(row) != size[0] {
					t.Fatalf("size=%v row escapes pane: %q", size, row)
				}
			}
		}
	}
}

func seedDocsCache(t *testing.T, home string) {
	t.Helper()
	data, err := json.Marshal(docsCatalog())
	if err != nil {
		t.Fatal(err)
	}
	// macOS uses HOME/Library/Caches; Linux uses the explicit XDG path below.
	for _, root := range []string{filepath.Join(home, "Library", "Caches"), filepath.Join(home, "cache")} {
		dir := filepath.Join(root, "lyra", "docs")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "v2.26-index-1.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
