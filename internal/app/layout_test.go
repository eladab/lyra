package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPaneGeometryAndControlLabels(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {88, 24}, {60, 12}, {1, 1}} {
		l := measure(size[0], size[1])
		if l.editorCols < 1 || l.editorRows < 1 {
			t.Fatal(l)
		}
		if l.previewCols > 0 && l.editorCols+l.previewCols+1 != l.width {
			t.Fatal(l)
		}
		if got := ansi.StringWidth(fit("界éabcdefgh", l.width)); got != l.width {
			t.Fatalf("width %d != %d", got, l.width)
		}
	}
	if got := cleanLabel("score\x1b[2J\n.ly"); strings.ContainsAny(got, "\x1b\n") {
		t.Fatal("untrusted filename emitted control sequences")
	}
}
