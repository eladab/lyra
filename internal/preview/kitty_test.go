package preview

import (
	"strings"
	"testing"
)

func TestKittyBoundedChunksPlacementAndOwnedDelete(t *testing.T) {
	k := Kitty{ID: 123456}
	payload := strings.Repeat("ABCD", 3100)
	out := k.Draw(payload, 61, 3, 59, 27)
	if !strings.HasPrefix(out, "\x1b7\x1b[4;62H\x1b_Ga=T") || !strings.HasSuffix(out, "\x1b\\\x1b8") {
		t.Fatal("placement moved outside intended origin or failed to restore cursor")
	}
	var got strings.Builder
	for _, part := range strings.Split(out, "\x1b_G")[1:] {
		body, _, _ := strings.Cut(part, "\x1b\\")
		control, data, _ := strings.Cut(body, ";")
		if len(data) > 4096 {
			t.Fatal("oversized Kitty chunk")
		}
		if !strings.Contains(control, "q=2") {
			t.Fatal("graphics reply suppression missing")
		}
		got.WriteString(data)
	}
	if got.String() != payload {
		t.Fatal("chunked transfer corrupted PNG")
	}
	if !strings.Contains(out, "c=59,r=27,C=1") || strings.Contains(k.Delete(), "d=A") || !strings.Contains(k.Delete(), "d=I,i=123456") {
		t.Fatal("unbounded placement/delete")
	}
}
