// Package docs indexes the official LilyPond HTML manuals for offline
// lookup. Downloaded HTML is data, never terminal output or executable content.
package docs

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type Section struct {
	Title, Manual, URL, Text string
	Terms                    []string
	searchText               string
}

type Catalog struct {
	Version  string
	Sections []Section
}

// Prepare builds reusable full-text search data before handing the immutable
// catalog to the UI.
func (c *Catalog) Prepare() {
	for i := range c.Sections {
		c.Sections[i].searchText = strings.ToLower(c.Sections[i].Text)
	}
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}

func compact(s string) string { return strings.Join(strings.Fields(safe(s)), " ") }

// Texinfo encodes punctuation in index anchors as _hhhh hexadecimal escapes.
func indexTerm(id string) string {
	s := strings.TrimPrefix(id, "index-")
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '_' && i+5 <= len(s) {
			if r, err := strconv.ParseUint(s[i+1:i+5], 16, 16); err == nil {
				b.WriteRune(rune(r))
				i += 5
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return compact(b.String())
}

// Parse retains prose, indented examples, table rows, source attribution and
// links. It excludes the website's navigation, scripts, styles and forms.
func Parse(r io.Reader, manual, source string) ([]Section, error) {
	root, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	var sections []Section
	var body strings.Builder
	var line strings.Builder
	var current *Section
	anchor := ""
	flush := func() {
		if s := compact(line.String()); s != "" {
			body.WriteString(s + "\n")
		}
		line.Reset()
	}
	finish := func() {
		flush()
		if current != nil {
			current.Text = strings.Trim(body.String(), "\n")
			sections = append(sections, *current)
		}
		body.Reset()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch attr(n, "id") {
			case "footer", "tocframe", "highlighting-settings":
				return
			}
			class := attr(n, "class")
			switch n.Data {
			case "head", "script", "style", "nav", "form":
				return
			}
			for _, c := range strings.Fields(class) {
				switch c {
				case "nav_table", "header", "menu", "contents", "toc", "sidebar", "copiable-anchor":
					return
				}
			}
			id := attr(n, "id")
			if id != "" && !strings.HasPrefix(id, "index-") {
				anchor = id
			}
			if strings.HasPrefix(id, "index-") && current != nil {
				current.Terms = append(current.Terms, indexTerm(id))
			}
			if n.Data == "h1" || n.Data == "h2" || n.Data == "h3" || n.Data == "h4" {
				finish()
				u := *base
				u.Fragment = anchor
				current = &Section{Title: compact(textContent(n)), Manual: manual, URL: u.String()}
				return
			}
			if current == nil {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				return
			}
			switch n.Data {
			case "pre":
				flush()
				body.WriteByte('\n')
				for _, row := range strings.Split(strings.Trim(safe(textContent(n)), "\n"), "\n") {
					body.WriteString("    " + row + "\n")
				}
				body.WriteByte('\n')
				return
			case "img":
				line.WriteString(" [" + compact(attr(n, "alt")) + "] ")
				return
			case "br":
				flush()
				return
			case "p", "div", "tr", "blockquote", "ul", "ol", "dl", "h5":
				flush()
			case "li", "dt":
				flush()
				line.WriteString("• ")
			case "td", "th":
				line.WriteString("  ")
			}
		}
		if n.Type == html.TextNode && current != nil {
			line.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "tr", "li", "dt", "dd", "h5":
				flush()
			case "a":
				// Keep links readable without executing or emitting website escapes.
				if href := attr(n, "href"); href != "" && !strings.HasPrefix(href, "#") {
					if ref, err := url.Parse(href); err == nil && (ref.Scheme == "" || ref.Scheme == "https" || ref.Scheme == "http") {
						line.WriteString(" (" + base.ResolveReference(ref).String() + ")")
					}
				}
			}
		}
	}
	walk(root)
	finish()
	if len(sections) < 1 {
		return nil, fmt.Errorf("%s manual contains no documentation sections", manual)
	}
	return sections, nil
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimLeft(strings.TrimSpace(s), "\\"))
}

// Search ranks indexed command matches first, then titles, fuzzy title/term
// matches and full text. The catalog stays immutable while the UI searches it.
func (c *Catalog) Search(query string, limit int) []int {
	q := normalize(query)
	type match struct{ index, score int }
	var matches []match
	for i, s := range c.Sections {
		title := normalize(s.Title)
		bareTitle := strings.TrimLeft(title, "0123456789. ")
		score := 0
		switch {
		case q == "":
			score = 1
		case bareTitle == q:
			score = 1200
		case strings.Contains(title, q):
			score = 700
		case fuzzy(q, title):
			score = 300
		}
		for _, term := range s.Terms {
			t := normalize(term)
			if t == q {
				bonus := 0
				if strings.Contains(title, q) {
					bonus = 50
				}
				score = max(score, 1000+bonus)
			} else if q != "" && strings.Contains(t, q) {
				score = max(score, 600)
			} else if q != "" && fuzzy(q, t) {
				score = max(score, 250)
			}
		}
		if score == 0 {
			all := s.searchText
			if all == "" {
				all = strings.ToLower(s.Text)
			}
			found := true
			for _, word := range strings.Fields(q) {
				if !strings.Contains(all, word) {
					found = false
					break
				}
			}
			if found {
				score = 100
			}
		}
		if score > 0 {
			matches = append(matches, match{i, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	var result []int
	for _, m := range matches[:min(max(0, limit), len(matches))] {
		result = append(result, m.index)
	}
	return result
}

func fuzzy(needle, haystack string) bool {
	q := []rune(needle)
	i := 0
	for _, r := range haystack {
		if i < len(q) && r == q[i] {
			i++
		}
	}
	return i == len(q)
}
