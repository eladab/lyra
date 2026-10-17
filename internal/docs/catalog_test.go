package docs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// Small authored fixture reflecting Texinfo's old and new heading/anchor
// layouts, not copied manual text. Integration can also use downloaded HTML.
const fixture = `<html><head><title>LilyPond</title><style>BAD STYLE</style></head><body>
<div id="main"><span id="top"></span><h1>LilyPond reference</h1>
<table class="menu"><tr><td>BAD MENU</td></tr></table>
<div class="subsection" id="tuplets"><table class="nav_table"><tr><td>BAD NAV</td></tr></table>
<span id="Tuplets"></span><h4>2.1.2 Tuplets</h4><span id="index-_005ctuplet"></span>
<p>Group notes &amp; rhythms with <code>\tuplet</code>.</p>
<pre><span>\tuplet 3/2 {</span>
  c8 d e
}</pre><p>Read <a href="learning/tutorial.html">the tutorial</a>.</p>
<img src="music.png" alt="image of music"></div>
<div id="relative"><h3>Relative pitches</h3><span id="index-_005crelative"></span>
<p>This is about octaves.</p><script>BAD SCRIPT</script>
<p>controls &#27;[2J must be inert</p></div>
<div class="contents"><h2>BAD CONTENTS</h2></div>
<div id="footer">BAD FOOTER</div><div id="tocframe"><h4>BAD TOC</h4></div>
</div></body></html>`

func TestParseAndRankOfficialManualStructure(t *testing.T) {
	sections, err := Parse(strings.NewReader(fixture), "Notation Reference", "https://lilypond.org/doc/v2.26/Documentation/notation-big-page.html")
	if err != nil || len(sections) != 3 {
		t.Fatalf("parse: %d sections, %v", len(sections), err)
	}
	c := &Catalog{Version: "2.26", Sections: sections}
	c.Prepare()
	for _, query := range []string{`\tuplet`, "tuplet", "tplts", "group notes"} {
		got := c.Search(query, 20)
		if len(got) == 0 || got[0] != 1 {
			t.Fatalf("%q lookup: %+v", query, got)
		}
	}
	for _, want := range []string{"Group notes & rhythms", "    \\tuplet 3/2 {\n      c8 d e\n    }", "https://lilypond.org/doc/v2.26/Documentation/learning/tutorial.html", "[image of music]"} {
		if !strings.Contains(sections[1].Text, want) {
			t.Errorf("missing %q in %q", want, sections[1].Text)
		}
	}
	if !strings.HasSuffix(sections[1].URL, "#Tuplets") || !strings.HasSuffix(sections[2].URL, "#relative") {
		t.Fatal("section anchors lost", sections)
	}
	for _, s := range sections {
		if strings.Contains(s.Text, "BAD") || strings.ContainsRune(s.Text, 0x1b) {
			t.Fatal("website chrome or escapes entered documentation", s)
		}
	}
	if got := c.Search("no-such-topic-xyz", 20); len(got) != 0 {
		t.Fatal("unexpected result", got)
	}
	if got := c.Search("", 2); len(got) != 2 {
		t.Fatal("result limit ignored", got)
	}
}

func TestCodeAtStartOfSectionKeepsIndentation(t *testing.T) {
	sections, err := Parse(strings.NewReader("<h3 id='code'>Example</h3><pre>  c4\n    d4</pre>"), "Learning", "https://lilypond.org/manual")
	if err != nil || len(sections) != 1 || sections[0].Text != "      c4\n        d4" {
		t.Fatalf("leading code indentation changed: %+v %v", sections, err)
	}
}

func manualFixture() string {
	var b strings.Builder
	b.WriteString(fixture)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, `<h3 id="topic-%d">Topic %d</h3><p>Some useful explanation.</p>`, i, i)
	}
	return b.String()
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLoadCacheOfflineAndFailedRefresh(t *testing.T) {
	var requests atomic.Int32
	var fail atomic.Bool
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if fail.Load() {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(manualFixture()))}, nil
	})}
	l := Loader{Version: "2.26", CacheDir: t.TempDir(), BaseURL: "https://lilypond.example/", Client: client}
	c, err := l.Load(context.Background(), false)
	if err != nil || c == nil || len(c.Sections) < 20 || requests.Load() != 2 {
		t.Fatalf("download: %+v %v requests=%d", c, err, requests.Load())
	}
	fail.Store(true)
	if _, err := l.Load(context.Background(), true); err == nil {
		t.Fatal("failed refresh reported success")
	}
	before := requests.Load()
	c, err = l.Load(context.Background(), false)
	if err != nil || c == nil || len(c.Search("tuplet", 10)) == 0 || requests.Load() != before {
		t.Fatalf("cached offline lookup failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(l.CacheDir, "v2.26-index-1.json"))
	if err != nil || len(data) < 1000 {
		t.Fatal("cache missing", err)
	}
	if err := os.WriteFile(filepath.Join(l.CacheDir, "v2.26-index-1.json"), []byte("broken json"), 0600); err != nil {
		t.Fatal(err)
	}
	fail.Store(false)
	if _, err := l.Load(context.Background(), false); err != nil || requests.Load() != before+2 {
		t.Fatal("corrupt cache was not rebuilt", err)
	}
}

func TestRejectInvalidVersionsAndErrorPages(t *testing.T) {
	for _, v := range []string{"../../elsewhere", "2.26/", "stable", "2.26.0", "3.0"} {
		if _, err := (Loader{Version: v, CacheDir: t.TempDir()}).Load(context.Background(), false); err == nil {
			t.Fatal("accepted invalid version", v)
		}
	}
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<h1>Service unavailable</h1>"))}, nil
	})}
	l := Loader{CacheDir: t.TempDir(), Client: client}
	if _, err := l.Load(context.Background(), false); err == nil {
		t.Fatal("error page indexed as a manual")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Load(ctx, false); err == nil {
		t.Fatal("cancelled load succeeded")
	}
}

func TestDownloadedOfficialManual(t *testing.T) {
	path := os.Getenv("LYRA_TEST_DOCS_HTML")
	if path == "" {
		t.Skip("set LYRA_TEST_DOCS_HTML to a downloaded notation-big-page.html")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Parse(f, "Notation Reference", "https://lilypond.org/doc/v2.26/Documentation/notation-big-page.html")
	if err != nil || len(s) < 300 {
		t.Fatalf("official manual: sections=%d err=%v", len(s), err)
	}
	c := &Catalog{Version: "2.26", Sections: s}
	c.Prepare()
	for query, title := range map[string]string{"tuplet": "Tuplets", "relative": "Relative octave", "clef": "Clef"} {
		results := c.Search(query, 10)
		if len(results) == 0 || !strings.Contains(c.Sections[results[0]].Title, title) {
			t.Fatalf("%q: got %+v", query, results)
		}
		t.Logf("%s -> %s (%s)", query, c.Sections[results[0]].Title, c.Sections[results[0]].URL)
	}
	t.Logf("indexed %d real manual sections", len(s))
	if learning := os.Getenv("LYRA_TEST_DOCS_LEARNING_HTML"); learning != "" {
		client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			file := path
			if strings.Contains(r.URL.Path, "learning-big-page") {
				file = learning
			}
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Body: f}, nil
		})}
		loader := Loader{CacheDir: t.TempDir(), Client: client}
		both, err := loader.Load(context.Background(), false)
		if err != nil || both == nil || len(both.Sections) <= len(s)+100 {
			t.Fatalf("real two-manual download/index/cache path: %v", err)
		}
		offline, err := loader.Load(context.Background(), false)
		if err != nil || len(offline.Sections) != len(both.Sections) {
			t.Fatal("real two-manual offline index", err)
		}
		t.Logf("indexed/cached %d sections from both real manuals", len(both.Sections))
	}
}
