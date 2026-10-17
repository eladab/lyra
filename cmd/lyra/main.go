package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lyra/internal/app"
	documentation "lyra/internal/docs"
	"lyra/internal/preview"
)

func parse(args []string, output io.Writer) (app.Options, error) {
	opts := app.Options{}
	previewTheme, vimStyle := "warm", "lyra"
	flags := flag.NewFlagSet("lyra", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&opts.Vim, "vim", "vim", "traditional Vim executable (no shell arguments)")
	flags.StringVar(&opts.LilyPond, "lilypond", "lilypond", "LilyPond executable (editing works without it)")
	flags.StringVar(&opts.PDFInfo, "pdfinfo", "pdfinfo", "Poppler pdfinfo executable")
	flags.StringVar(&opts.PDFToPPM, "pdftoppm", "pdftoppm", "Poppler pdftoppm executable")
	flags.StringVar(&opts.FluidSynth, "fluidsynth", "fluidsynth", "FluidSynth executable")
	flags.StringVar(&opts.MPV, "mpv", "mpv", "mpv audio player executable")
	flags.StringVar(&opts.SoundFont, "soundfont", "", "SoundFont bank (default: LYRA_SOUNDFONT or installed GeneralUser GS)")
	flags.StringVar(&opts.AudioOutput, "audio-output", "", "mpv audio output driver (default: platform default; e.g. coreaudio)")
	flags.StringVar(&previewTheme, "preview-theme", "warm", "PDF appearance: warm, night, or paper")
	flags.StringVar(&vimStyle, "vim-style", "lyra", "LilyPond syntax palette: lyra or user (keep existing Vim syntax)")
	flags.StringVar(&opts.DocsVersion, "docs-version", documentation.DefaultVersion, "LilyPond documentation series to cache (e.g. 2.26 or 2.24)")
	flags.BoolVar(&opts.DisableDocsKey, "no-docs-key", false, "keep Vim's default K keyword program; :LyraDocs and prefix D still work")
	flags.DurationVar(&opts.BuildTimeout, "build-timeout", 2*time.Minute, "maximum duration of one LilyPond build")
	flags.Func("include", "additional LilyPond include directory (repeatable)", func(dir string) error {
		opts.IncludeDirs = append(opts.IncludeDirs, dir)
		return nil
	})
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: lyra [options] [composition.ly]\n\nLyra: embedded Vim, LilyPond builds, PDF preview and MIDI playback.\nWithout a filename, browse for a score or choose where to create one.\nCtrl+\\ then Space plays/pauses; a opens audio; p previews; h opens help.\n:q or :wq exits through Vim.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return opts, fmt.Errorf("expected at most one LilyPond source file")
	}
	opts.File = flags.Arg(0)
	if opts.BuildTimeout <= 0 {
		return opts, fmt.Errorf("--build-timeout must be positive")
	}
	if !documentation.ValidVersion(opts.DocsVersion) {
		return opts, fmt.Errorf("--docs-version must be a LilyPond series such as 2.26")
	}
	theme, err := preview.ParseTheme(previewTheme)
	if err != nil {
		return opts, err
	}
	opts.PreviewTheme = theme
	switch vimStyle {
	case "lyra":
	case "user":
		opts.DisableLilyPondSyntax = true
	default:
		return opts, fmt.Errorf("--vim-style must be lyra or user")
	}
	return opts, nil
}

func main() {
	opts, err := parse(os.Args[1:], os.Stderr)
	if err == flag.ErrHelp {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lyra:", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	err = app.Run(ctx, opts, os.Stdin, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lyra:", err)
		os.Exit(1)
	}
}
