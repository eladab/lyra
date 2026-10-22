// Package scoretemplate provides offline score starters from the official
// LilyPond Learning Manual and Lyra's ensemble library, with forgiving search.
package scoretemplate

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

//go:embed templates.json
var data []byte

// Keep original ensemble sources separate from the generated manual catalog.
//
//go:embed ensembles.json ensembles/*.ly
var ensembles embed.FS

type Template struct {
	ID, Category, Name, Source, Aliases, Code string
}

func (t Template) Label() string {
	if t.Category == "" {
		return t.Name
	}
	return t.Category + " · " + t.Name
}

var catalog = load()

func load() []Template {
	all := []Template{{ID: "blank", Name: "Blank score", Aliases: "empty none no template"}}
	var imported []Template
	if err := json.Unmarshal(data, &imported); err != nil {
		panic("invalid embedded LilyPond templates: " + err.Error())
	}
	all = append(all, imported...)
	metadata, err := ensembles.ReadFile("ensembles.json")
	if err != nil {
		panic(err)
	}
	var additions []Template
	if err := json.Unmarshal(metadata, &additions); err != nil {
		panic("invalid embedded ensemble templates: " + err.Error())
	}
	for i := range additions {
		code, err := ensembles.ReadFile("ensembles/" + additions[i].ID + ".ly")
		if err != nil {
			panic(err)
		}
		additions[i].Code = string(code)
	}
	return append(all, additions...)
}

func words(text string) []string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, text)
	result := strings.Fields(text)
	for i, word := range result {
		switch word {
		case "staff", "staves", "stave", "staffs":
			result[i] = "staff"
		}
	}
	return result
}

// Search matches words in any order across the category, topic and aliases.
// Exact words outrank prefixes, subsequences and one-character spelling errors.
func Search(query string) []Template {
	needles := words(query)
	type match struct {
		t     Template
		score int
	}
	var matches []match
	for _, t := range catalog {
		label := words(t.Label())
		aliases := words(t.Aliases)
		score, found := 0, true
		for _, needle := range needles {
			best := 0
			for _, token := range label {
				best = max(best, tokenScore(needle, token))
			}
			for _, token := range aliases {
				if s := tokenScore(needle, token); s > 0 {
					best = max(best, s-5)
				}
			}
			if best == 0 {
				found = false
				break
			}
			score += best
		}
		if found {
			if len(needles) > 0 && strings.Join(words(t.Name), " ") == strings.Join(needles, " ") {
				score += 100
			}
			matches = append(matches, match{t, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if len(needles) > 0 {
			return len(matches[i].t.Label()) < len(matches[j].t.Label())
		}
		return false
	})
	result := make([]Template, len(matches))
	for i, match := range matches {
		result[i] = match.t
	}
	return result
}

func tokenScore(needle, token string) int {
	if needle == token {
		return 60
	}
	if strings.HasPrefix(token, needle) {
		return 45
	}
	n, t := []rune(needle), []rune(token)
	i := 0
	for _, r := range t {
		if i < len(n) && n[i] == r {
			i++
		}
	}
	if i == len(n) {
		return max(10, 30-(len(t)-len(n)))
	}
	if len(n) >= 4 && near(n, t) {
		return 15
	}
	return 0
}

// near allows one insertion, deletion, replacement or adjacent transposition.
func near(a, b []rune) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	i := 0
	for i < len(a) && a[i] == b[i] {
		i++
	}
	if i == len(a) {
		return true
	}
	if len(a) == len(b) {
		if string(a[i+1:]) == string(b[i+1:]) {
			return true
		}
		return i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && string(a[i+2:]) == string(b[i+2:])
	}
	return string(a[i:]) == string(b[i+1:])
}
