//go:build darwin || linux

package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"lyra/internal/input"
	"lyra/internal/lilypond"
)

func TestBuildPaneDiagnosticsAndOpaqueOutput(t *testing.T) {
	m := &model{file: "score.ly", layout: measure(120, 20), focus: input.Preview,
		buildState: lilypond.State{Status: "Failed", Generation: 2,
			LastGood: &lilypond.Result{Generation: 1, PDFs: []string{"/tmp/previous.pdf"}},
			Latest:   &lilypond.Result{Err: errors.New("build failed"), Log: "bad\x1b[2J\nlog", Diagnostics: []lilypond.Diagnostic{{File: "part:one.ily", Line: 12, Column: 3, Severity: "error", Message: "invalid\x1b[2J\n  detail"}}}}}
	for _, log := range []bool{false, true} {
		m.showLog = log
		lines := m.buildLines(40)
		for _, line := range lines {
			if strings.ContainsAny(line, "\x1b\r\n\x00") || ansi.StringWidth(line) > 40 {
				t.Fatalf("untrusted output escaped its pane: %q", line)
			}
		}
		all := strings.Join(lines, "\n")
		if !strings.Contains(all, "Last good: generation 1") || (!log && !strings.Contains(all, "part:one.ily:12:3")) {
			t.Fatal("lost retained output or source diagnostics")
		}
	}
	if err := m.dispatch([]input.Event{{Data: []byte("G")}}); err != nil {
		t.Fatal(err)
	}
	if m.paneScroll == 0 {
		t.Fatal("long build pane cannot scroll")
	}
	if err := m.dispatch([]input.Event{{Data: []byte("g")}}); err != nil {
		t.Fatal(err)
	}
	if m.paneScroll != 0 {
		t.Fatal("cannot return to top of diagnostics")
	}
}
