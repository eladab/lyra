package input

import (
	"bytes"
	"testing"
)

func TestOpaqueInputAndChunkedPaste(t *testing.T) {
	want := []byte("\t\x03\x13\x11é♩\x1b[A\x1b[200~literal\x1cq\x1cp\x1b[201~")
	for split := 0; split <= len(want); split++ {
		var r Router
		events := append(r.Feed(want[:split]), r.Feed(want[split:])...)
		events = append(events, r.Flush()...)
		var got []byte
		for _, e := range events {
			if e.Action != None {
				t.Fatalf("split %d: paste became action %d", split, e.Action)
			}
			got = append(got, e.Data...)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("split %d: %q != %q", split, got, want)
		}
	}
}

func TestPrefixAcrossReads(t *testing.T) {
	var r Router
	if got := r.Feed([]byte{Prefix}); len(got) != 0 {
		t.Fatal(got)
	}
	got := r.Feed([]byte{'p', Prefix, Prefix, Prefix, 'x', Prefix, 'e'})
	if len(got) != 3 || got[0].Action != Preview || !bytes.Equal(got[1].Data, []byte{Prefix, Prefix, 'x'}) || got[2].Action != Editor {
		t.Fatalf("unexpected routing: %+v", got)
	}
}

func TestBuildControlsPreserveOrdinaryInputAndPaste(t *testing.T) {
	var r Router
	events := r.Feed([]byte("bc\x1cb\x1cc\x1b[200~\x1cb\x1cc\x1b[201~"))
	var data []byte
	var actions []Action
	for _, event := range events {
		data = append(data, event.Data...)
		if event.Action != None {
			actions = append(actions, event.Action)
		}
	}
	if string(data) != "bc\x1b[200~\x1cb\x1cc\x1b[201~" || len(actions) != 2 || actions[0] != Build || actions[1] != CancelBuild {
		t.Fatalf("input or paste changed: %q %+v", data, actions)
	}
}

func TestLoneEscapeFlush(t *testing.T) {
	var r Router
	if got := r.Feed([]byte{27}); len(got) != 0 {
		t.Fatal(got)
	}
	got := r.Flush()
	if len(got) != 1 || !bytes.Equal(got[0].Data, []byte{27}) {
		t.Fatal(got)
	}
}

func TestPasteOneByteAtATime(t *testing.T) {
	data := []byte("\x1b[200~\x1cq\x1cp\x1b[201~\x1ce")
	var r Router
	var got []byte
	var actions []Action
	for _, b := range data {
		for _, e := range r.Feed([]byte{b}) {
			got = append(got, e.Data...)
			if e.Action != None {
				actions = append(actions, e.Action)
			}
		}
	}
	if string(got) != string(data[:len(data)-2]) || len(actions) != 1 || actions[0] != Editor {
		t.Fatalf("%q %+v", got, actions)
	}
}

func TestPlaybackControlsPreserveOrdinaryKeysAndPaste(t *testing.T) {
	data := []byte(" as,.u\x1c \x1cs\x1c,\x1c.\x1cu\x1ca\x1b[200~\x1c \x1cs\x1cu\x1b[201~")
	for split := 0; split <= len(data); split++ {
		var r Router
		events := append(r.Feed(data[:split]), r.Feed(data[split:])...)
		events = append(events, r.Flush()...)
		var keys []byte
		var actions []Action
		for _, e := range events {
			if e.Action == None {
				keys = append(keys, e.Data...)
			} else {
				actions = append(actions, e.Action)
			}
		}
		want := []Action{PlayPause, StopAudio, SeekBack, SeekForward, LatestAudio, Audio}
		if string(keys) != " as,.u\x1b[200~\x1c \x1cs\x1cu\x1b[201~" || len(actions) != len(want) {
			t.Fatalf("split %d: %q %+v", split, keys, actions)
		}
		for i, a := range want {
			if actions[i] != a {
				t.Fatal(actions)
			}
		}
	}
}

func TestThemePrefixPreservesVimKeysAndPaste(t *testing.T) {
	data := []byte("t\x1ct\x1b[200~t\x1ct\x1b[201~")
	for split := 0; split <= len(data); split++ {
		var r Router
		events := append(r.Feed(data[:split]), r.Feed(data[split:])...)
		events = append(events, r.Flush()...)
		var keys []byte
		themes := 0
		for _, event := range events {
			if event.Action == PreviewTheme {
				themes++
			} else if event.Action == None {
				keys = append(keys, event.Data...)
			} else {
				t.Fatal(event.Action)
			}
		}
		if themes != 1 || string(keys) != "t\x1b[200~t\x1ct\x1b[201~" {
			t.Fatalf("split %d: theme shortcut leaked into Vim or paste: %q %d", split, keys, themes)
		}
	}
}
