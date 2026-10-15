package main

import (
	"bytes"
	"flag"
	"testing"

	"lyra/internal/preview"
)

func TestCLI(t *testing.T) {
	for _, args := range [][]string{nil, {"--preview-theme", "night"}, {"composition.ly"}, {"--vim", "/usr/bin/vim", "score with spaces.ly"}, {"--", "-score.ly"}, {"--lilypond", "/custom/lilypond", "--include", "parts", "--include", "shared", "--build-timeout", "30s", "score.ly"}} {
		if _, err := parse(args, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"one.ly", "two.ly"}, {"--unknown"}, {"--build-timeout", "0", "score.ly"}, {"--build-timeout", "-1s", "score.ly"}, {"--preview-theme", "sepia", "score.ly"}, {"--vim-style", "unknown", "score.ly"}} {
		if _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Fatal(args)
		}
	}
	if _, err := parse([]string{"--help"}, &bytes.Buffer{}); err != flag.ErrHelp {
		t.Fatal(err)
	}
	opts, err := parse([]string{"--soundfont", "/bank with spaces.sf2", "--fluidsynth", "/custom/fluid", "--mpv", "/custom/player", "--audio-output", "coreaudio", "score.ly"}, &bytes.Buffer{})
	if err != nil || opts.SoundFont != "/bank with spaces.sf2" || opts.FluidSynth != "/custom/fluid" || opts.MPV != "/custom/player" || opts.AudioOutput != "coreaudio" {
		t.Fatalf("audio flags: %+v %v", opts, err)
	}
}

func TestAppearanceOptions(t *testing.T) {
	opts, err := parse([]string{"score.ly"}, &bytes.Buffer{})
	if err != nil || opts.PreviewTheme != preview.ThemeWarm || opts.DisableLilyPondSyntax {
		t.Fatalf("default appearance: %+v %v", opts, err)
	}
	opts, err = parse([]string{"--preview-theme", "night", "--vim-style", "user", "score.ly"}, &bytes.Buffer{})
	if err != nil || opts.PreviewTheme != preview.ThemeNight || !opts.DisableLilyPondSyntax {
		t.Fatalf("custom appearance: %+v %v", opts, err)
	}
}
