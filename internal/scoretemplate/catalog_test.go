package scoretemplate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindTemplatesByTopicPathAliasesAndSpelling(t *testing.T) {
	for query, want := range map[string]string{
		"": "blank", "empty": "blank", "single staff notes only": "notes-only",
		"single-staff notes": "notes-only", "staff single only": "notes-only",
		"str qrt": "string-quartet", "quartet string": "string-quartet",
		"string qaurtet": "string-quartet", "string quartet parts": "string-quartet-parts",
		"violins viola cello": "string-quartet", "solo piano": "solo-piano",
		"PIANO SOLO": "solo-piano", "vocal satb reduction": "satb-vocal-score-and-automatic-piano-reduction",
		"lead sheet chords lyrics": "notes-lyrics-and-chords",
		"gregorian chant":          "gregorian-transcription-template", "orchestra choir": "orchestra-choir-and-piano",
	} {
		got := Search(query)
		if len(got) == 0 || got[0].ID != want {
			t.Errorf("%q: wanted %s, got %+v", query, want, labels(got))
		}
	}
	if got := Search("unfindablexyz"); len(got) != 0 {
		t.Fatal("unrelated templates matched", labels(got))
	}
	all := Search("")
	if len(all) != 23 || all[0].Code != "" {
		t.Fatal("expected blank plus every template topic", len(all))
	}
	seen := map[string]bool{}
	for _, template := range all[1:] {
		if seen[template.ID] || template.Category == "" || template.Code == "" || !strings.HasPrefix(template.Source, "https://lilypond.org/") {
			t.Fatal("invalid embedded template", template.ID)
		}
		seen[template.ID] = true
		if !strings.Contains(template.Code, `\version "2.26.0"`) || !strings.Contains(template.Code, template.Source) {
			t.Fatal("missing source/version", template.ID)
		}
		// Every displayed topic is reachable by its concatenated path.
		found := false
		for _, result := range Search(template.Label()) {
			found = found || result.ID == template.ID
		}
		if !found {
			t.Fatal("cannot find full template name", template.Label())
		}
	}
}

func labels(items []Template) []string {
	var result []string
	for _, item := range items {
		result = append(result, item.Label())
	}
	return result
}

func TestRealLilyPondEngravesEveryTemplate(t *testing.T) {
	exe := os.Getenv("LYRA_TEST_LILYPOND")
	if exe == "" {
		t.Skip("set LYRA_TEST_LILYPOND to engrave all bundled templates")
	}
	for _, template := range Search("")[1:] {
		t.Run(template.ID, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "new score.ly")
			if err := os.WriteFile(source, []byte(template.Code), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "--pdf", "-dno-point-and-click", "--output="+filepath.Join(dir, "score"), source)
			cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("template does not engrave: %v\n%s", err, output)
			}
			pages, _ := filepath.Glob(filepath.Join(dir, "score*.pdf"))
			if len(pages) == 0 {
				t.Fatal("successful process produced no PDF")
			}
		})
	}
}
