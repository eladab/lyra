package scoretemplate

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEnsembleInstrumentation(t *testing.T) {
	// Independent players/sections in score order; keyboard hands are separate
	// staves. In particular, neither quintet variant may silently omit a part.
	stringsSection := "violinOne violinTwo viola cello doubleBass"
	doubleWinds := "fluteOne fluteTwo oboeOne oboeTwo clarinetOne clarinetTwo bassoonOne bassoonTwo"
	want := map[string]string{
		"string-trio":                     "violin viola cello",
		"string-quintet-two-violas":       "violinOne violinTwo violaOne violaTwo cello",
		"string-quintet-two-cellos":       "violinOne violinTwo viola celloOne celloTwo",
		"string-sextet":                   "violinOne violinTwo violaOne violaTwo celloOne celloTwo",
		"piano-trio":                      "violin cello pianoRight pianoLeft",
		"piano-quartet":                   "violin viola cello pianoRight pianoLeft",
		"piano-quintet":                   "violinOne violinTwo viola cello pianoRight pianoLeft",
		"clarinet-quintet":                "clarinet violinOne violinTwo viola cello",
		"woodwind-quintet":                "flute oboe clarinet horn bassoon",
		"brass-quintet":                   "trumpetOne trumpetTwo horn trombone tuba",
		"string-orchestra":                stringsSection,
		"chamber-orchestra-single-winds":  "flute oboe clarinet bassoon hornOne hornTwo " + stringsSection,
		"chamber-orchestra-classical":     doubleWinds + " hornOne hornTwo trumpetOne trumpetTwo timpani " + stringsSection,
		"symphony-orchestra-double-winds": doubleWinds + " hornOne hornTwo hornThree hornFour trumpetOne trumpetTwo tromboneOne tromboneTwo bassTrombone tuba timpani " + stringsSection,
		"symphony-orchestra-expanded": "piccolo fluteOne fluteTwo oboeOne oboeTwo englishHorn clarinetOne clarinetTwo bassClarinet bassoonOne bassoonTwo contrabassoon " +
			"hornOne hornTwo hornThree hornFour trumpetOne trumpetTwo trumpetThree tromboneOne tromboneTwo bassTrombone tuba timpani snare harpRight harpLeft " + stringsSection,
	}
	staff := regexp.MustCompile(`\\new (?:Drum)?Staff = "([^"]+)"`)
	manuals := 0
	for _, template := range Search("")[1:] {
		if template.Source != "Lyra ensemble library" {
			if !strings.HasPrefix(template.Source, "https://lilypond.org/") {
				t.Fatal("missing manual attribution", template.ID)
			}
			manuals++
			continue
		}
		var got []string
		for _, match := range staff.FindAllStringSubmatch(template.Code, -1) {
			got = append(got, match[1])
		}
		if !slices.Equal(got, strings.Fields(want[template.ID])) {
			t.Errorf("%s: staves %v, want %s", template.ID, got, want[template.ID])
		}
		delete(want, template.ID)
	}
	if manuals != 22 || len(want) != 0 {
		t.Fatalf("missing catalog entries: %d manual topics, missing ensembles %v", manuals, want)
	}
	files, err := ensembles.ReadDir("ensembles")
	if err != nil || len(files) != 15 {
		t.Fatal("ensemble files and catalog differ", len(files), err)
	}
}

func TestRealEnsembleMIDIPitchesAndInstruments(t *testing.T) {
	exe := os.Getenv("LYRA_TEST_LILYPOND")
	if exe == "" {
		t.Skip("set LYRA_TEST_LILYPOND to verify ensemble MIDI transpositions")
	}
	// Replace each rest-only pitched part with a distinct absolute test pitch.
	// These synthetic probes deliberately ignore instrument ranges: they detect
	// dropped staves, double transposition and octave errors in the final MIDI.
	part := regexp.MustCompile(`(?s)([a-zA-Z]+)Music = \\relative [a-z',]+ \{\s*\\global\s*R1\*4\s*\\bar "\|\."\s*\}`)
	names := []string{"c", "cis", "d", "dis", "e", "f", "fis", "g", "gis", "a", "ais", "b"}
	for _, template := range Search("") {
		if template.Source != "Lyra ensemble library" {
			continue
		}
		t.Run(template.ID, func(t *testing.T) {
			want := map[byte]byte{}
			code := part.ReplaceAllStringFunc(template.Code, func(block string) string {
				id := part.FindStringSubmatch(block)[1]
				i := len(want)
				want[byte(60+i)] = ensembleProgram(t, id, template.Category == "Orchestral")
				note := names[i%12] + strings.Repeat("'", 1+i/12)
				return fmt.Sprintf("%sMusic = { \\global %s1 R1*3 \\bar \"|.\" }", id, note)
			})
			if len(want) == 0 {
				t.Fatal("no pitched parts tested")
			}
			// A snare probe also verifies the percussion staff uses GM channel 10.
			drums := strings.Contains(code, "snareMusic =")
			if drums {
				code = strings.ReplaceAll(code, "  R1*4\n", "  sn1 R1*3\n")
			}
			dir := t.TempDir()
			source := filepath.Join(dir, "probe.ly")
			if err := os.WriteFile(source, []byte(code), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-dno-print-pages", "--output="+filepath.Join(dir, "probe"), source)
			cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"), "LC_ALL=C")
			if output, err := cmd.CombinedOutput(); err != nil || bytes.Contains(bytes.ToLower(output), []byte("warning:")) {
				t.Fatalf("MIDI probe failed: %v\n%s", err, output)
			}
			midi, err := os.ReadFile(filepath.Join(dir, "probe.midi"))
			if err != nil {
				t.Fatal(err)
			}
			channels := map[byte]byte{}
			for _, note := range readMIDINotes(t, midi) {
				if note.channel == 9 {
					if !drums || note.pitch != 38 {
						t.Fatalf("unexpected percussion note: %+v", note)
					}
					drums = false
					continue
				}
				if program, used := channels[note.channel]; used && program != note.program {
					t.Fatalf("MIDI channel %d reused for programs %d and %d", note.channel, program, note.program)
				}
				channels[note.channel] = note.program
				if program, ok := want[note.pitch]; !ok || program != note.program {
					t.Fatalf("wrong concert pitch or MIDI instrument: %+v, remaining %v", note, want)
				}
				delete(want, note.pitch)
			}
			if len(want) != 0 || drums {
				t.Fatalf("missing MIDI notes: %v, snare missing: %v", want, drums)
			}
		})
	}
}

func ensembleProgram(t *testing.T, id string, orchestral bool) byte {
	t.Helper()
	programs := map[string]byte{
		"piccolo": 72, "flute": 73, "oboe": 68, "englishHorn": 69,
		"clarinet": 71, "bassClarinet": 71, "bassoon": 70, "contrabassoon": 70,
		"horn": 60, "trumpet": 56, "trombone": 57, "bassTrombone": 57,
		"tuba": 58, "timpani": 47, "harp": 46, "piano": 0,
		"violin": 40, "viola": 41, "cello": 42, "doubleBass": 43,
	}
	if orchestral {
		for _, id := range []string{"violin", "viola", "cello", "doubleBass"} {
			programs[id] = 48
		}
	}
	for prefix, program := range programs {
		if strings.HasPrefix(id, prefix) {
			return program
		}
	}
	t.Fatalf("no expected MIDI instrument for %s", id)
	return 0
}

type midiNote struct{ pitch, program, channel byte }

// Decode note-on and program events from the real Standard MIDI File, including
// running status and variable-length delta times. No audio device is required.
func readMIDINotes(t *testing.T, data []byte) []midiNote {
	t.Helper()
	r := bytes.NewReader(data)
	read := func(n int) []byte {
		t.Helper()
		if n < 0 || n > r.Len() {
			t.Fatal("truncated MIDI")
		}
		b := make([]byte, n)
		_, _ = r.Read(b)
		return b
	}
	vlq := func() int {
		t.Helper()
		value := 0
		for range 4 {
			b := read(1)[0]
			value = value<<7 | int(b&0x7f)
			if b&0x80 == 0 {
				return value
			}
		}
		t.Fatal("invalid MIDI variable-length quantity")
		return 0
	}
	if string(read(4)) != "MThd" {
		t.Fatal("missing MIDI header")
	}
	read(int(binary.BigEndian.Uint32(read(4))))
	var notes []midiNote
	for r.Len() > 0 {
		if string(read(4)) != "MTrk" {
			t.Fatal("missing MIDI track")
		}
		length := int(binary.BigEndian.Uint32(read(4)))
		end := r.Len() - length
		var status byte
		var programs [16]byte
		for r.Len() > end {
			vlq() // delta time
			b := read(1)[0]
			if b < 0x80 {
				if status < 0x80 || status >= 0xf0 {
					t.Fatal("invalid running status")
				}
				_ = r.UnreadByte()
			} else {
				status = b
			}
			switch status {
			case 0xff:
				read(1) // meta-event type
				read(vlq())
			case 0xf0, 0xf7:
				read(vlq())
			default:
				channel := status & 0x0f
				kind := status & 0xf0
				first := read(1)[0]
				if kind == 0xc0 {
					programs[channel] = first
				} else if kind != 0xd0 {
					second := read(1)[0]
					if kind == 0x90 && second > 0 {
						notes = append(notes, midiNote{first, programs[channel], channel})
					}
				}
			}
		}
		if r.Len() != end {
			t.Fatal("invalid MIDI track length")
		}
	}
	return notes
}
