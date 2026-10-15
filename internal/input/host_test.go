package input

import (
	"bytes"
	"strings"
	"testing"
)

func TestHostRepliesAndMouseAcrossEveryBoundary(t *testing.T) {
	stream := []byte("i\x1b_Gi=123;OK\x1b\\\x1b[6;22;11t\x1b[4;880;1320t\x1b[<32;60;8M\x1b[<0;60;8mx")
	for split := 0; split <= len(stream); split++ {
		var r Router
		events := append(r.Feed(stream[:split]), r.Feed(stream[split:])...)
		events = append(events, r.Flush()...)
		var keys []byte
		replies, mouse := 0, 0
		for _, e := range events {
			switch e.Action {
			case None:
				keys = append(keys, e.Data...)
			case HostReply:
				replies++
				if e.Code == 6 && (e.X != 11 || e.Y != 22) {
					t.Fatal(e)
				}
			case MouseEvent:
				mouse++
				if e.X != 59 || e.Y != 7 {
					t.Fatal(e)
				}
			}
		}
		if string(keys) != "ix" || replies != 3 || mouse != 2 {
			t.Fatalf("split %d leaked replies: %q %+v", split, keys, events)
		}
	}
}

func TestRepliesCanInterruptPendingPrefixAndOversizedRepliesStayBounded(t *testing.T) {
	var r Router
	r.Feed([]byte{Prefix})
	events := r.Feed([]byte("\x1b_Gi=123;OK\x1b\\p"))
	if len(events) != 2 || events[0].Action != HostReply || events[1].Action != Preview {
		t.Fatalf("reply corrupted pending prefix: %+v", events)
	}
	r.Feed([]byte("\x1b_G" + strings.Repeat("x", 4093)))
	if len(r.pending) > 4096 {
		t.Fatal("unbounded host reply")
	}
	if events := r.Feed([]byte(strings.Repeat("x", 5000))); len(events) > 0 {
		t.Fatal("oversized reply leaked into Vim")
	}
	events = r.Feed([]byte("\x1b\\i"))
	if len(events) != 1 || events[0].Action != None || string(events[0].Data) != "i" {
		t.Fatalf("did not resume normal input: %+v", events)
	}
}
func TestHostRepliesStayOpaqueInPasteAndSlowRepliesDoNotLeak(t *testing.T) {
	want := []byte("\x1b[200~\x1b_Gi=123;OK\x1b\\\x1b[6;22;11t\x1b[<0;12;2M\x1c]\x1b[201~")
	var r Router
	var got []byte
	for _, b := range want {
		for _, e := range r.Feed([]byte{b}) {
			if e.Action != None {
				t.Fatalf("paste became host event: %+v", e)
			}
			got = append(got, e.Data...)
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatal("paste was changed")
	}
	r.Feed([]byte("\x1b_Gi=123;"))
	if len(r.Flush()) != 0 {
		t.Fatal("idle timeout forwarded partial graphics reply to Vim")
	}
	if events := r.Feed([]byte("OK\x1b\\")); len(events) != 1 || events[0].Action != HostReply {
		t.Fatal(events)
	}
}
