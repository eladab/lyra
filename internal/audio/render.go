//go:build darwin || linux

// Package audio prepares LilyPond MIDI asynchronously and controls a private mpv
// process. Neither synthesizer nor player owns terminal input or output.
package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	FluidSynth, MPV, SoundFont, Output string
	Timeout                            time.Duration
}

func DefaultSoundFont() string {
	if font := os.Getenv("LYRA_SOUNDFONT"); font != "" {
		return font
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lyra", "soundfonts", "GeneralUser-GS.sf2")
}

func resolveOptions(opts Options) (Options, error) {
	if opts.FluidSynth == "" {
		opts.FluidSynth = "fluidsynth"
	}
	if opts.MPV == "" {
		opts.MPV = "mpv"
	}
	if opts.SoundFont == "" {
		opts.SoundFont = DefaultSoundFont()
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}
	var err error
	for _, tool := range []*string{&opts.FluidSynth, &opts.MPV} {
		name := *tool
		*tool, err = exec.LookPath(name)
		if err != nil {
			return opts, fmt.Errorf("playback needs %s: run make deps or set --fluidsynth / --mpv", name)
		}
		*tool, err = filepath.Abs(*tool)
		if err != nil {
			return opts, err
		}
	}
	opts.SoundFont, err = filepath.Abs(opts.SoundFont)
	if err != nil {
		return opts, err
	}
	f, err := os.Open(opts.SoundFont)
	if err != nil {
		return opts, fmt.Errorf("SoundFont unavailable: use --soundfont /path/to/bank.sf2 or run make soundfont: %w", err)
	}
	defer f.Close()
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:4]) != "RIFF" || string(header[8:]) != "sfbk" {
		return opts, fmt.Errorf("invalid SoundFont: %s (expected a SoundFont RIFF sfbk bank)", opts.SoundFont)
	}
	return opts, nil
}

type Source struct {
	Generation uint64
	Open       func() (io.ReadCloser, error) // nil is a successful score with no MIDI
}
type take struct {
	generation uint64
	dir, wav   string
	duration   float64
}
type renderResult struct {
	take       *take
	generation uint64
	err        error
}

func render(ctx context.Context, opts Options, root string, source Source) renderResult {
	result := renderResult{generation: source.Generation}
	dir, err := os.MkdirTemp(root, "take-")
	if err != nil {
		result.err = err
		return result
	}
	good := false
	defer func() {
		if !good {
			_ = os.RemoveAll(dir)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	in, err := source.Open()
	if err != nil {
		result.err = err
		return result
	}
	defer in.Close()
	midi := filepath.Join(dir, "score.midi")
	out, err := os.Create(midi)
	if err != nil {
		result.err = err
		return result
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, (16<<20)+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > 16<<20 {
		result.err = errors.Join(copyErr, closeErr)
		if n > 16<<20 {
			result.err = fmt.Errorf("MIDI exceeds 16 MiB limit")
		}
		return result
	}
	if err := validateMIDI(midi); err != nil {
		result.err = err
		return result
	}
	wav := filepath.Join(dir, "score.wav")
	cmd := exec.CommandContext(ctx, opts.FluidSynth, "-ni", "-q", "-f", "/dev/null", "-F", wav, "-T", "wav", "-O", "s16", "-E", "little", "-r", "44100", "-g", "0.2", opts.SoundFont, midi)
	configureProcess(cmd)
	log := &boundedLog{}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		result.err = fmt.Errorf("start FluidSynth: %w", err)
		return result
	}
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }() // one Wait owner
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var waitErr error
	for {
		select {
		case waitErr = <-finished:
			goto complete
		case <-ticker.C:
			if info, err := os.Stat(wav); err == nil && info.Size() > 512<<20 {
				cancel()
				waitErr = <-finished
				result.err = fmt.Errorf("synthesized audio exceeds 512 MiB limit")
				goto complete
			}
		}
	}
complete:
	if result.err != nil {
		return result
	}
	if ctx.Err() != nil {
		result.err = ctx.Err()
		return result
	}
	if waitErr != nil {
		result.err = fmt.Errorf("FluidSynth: %w: %s", waitErr, log.String())
		return result
	}
	duration, err := waveDuration(wav)
	if err != nil {
		result.err = fmt.Errorf("FluidSynth output: %w: %s", err, log.String())
		return result
	}
	result.take = &take{generation: source.Generation, dir: dir, wav: wav, duration: duration}
	good = true
	return result
}

func validateMIDI(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	var header [14]byte
	if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:4]) != "MThd" || binary.BigEndian.Uint32(header[4:8]) != 6 || binary.BigEndian.Uint16(header[8:10]) > 2 || binary.BigEndian.Uint16(header[10:12]) == 0 || binary.BigEndian.Uint16(header[12:14]) == 0 {
		return fmt.Errorf("invalid LilyPond MIDI header")
	}
	for range binary.BigEndian.Uint16(header[10:12]) {
		var chunk [8]byte
		if _, err := io.ReadFull(f, chunk[:]); err != nil || string(chunk[:4]) != "MTrk" {
			return fmt.Errorf("invalid MIDI track")
		}
		pos, _ := f.Seek(0, io.SeekCurrent)
		length := int64(binary.BigEndian.Uint32(chunk[4:]))
		if length < 1 || pos+length > info.Size() {
			return fmt.Errorf("truncated MIDI track")
		}
		if _, err := f.Seek(length, io.SeekCurrent); err != nil {
			return err
		}
	}
	return nil
}

func waveDuration(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > 512<<20 {
		return 0, fmt.Errorf("WAV exceeds 512 MiB limit")
	}
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil || string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return 0, fmt.Errorf("missing PCM WAV output")
	}
	var byteRate uint32
	for range 1024 {
		var chunk [8]byte
		if _, err := io.ReadFull(f, chunk[:]); err != nil {
			return 0, fmt.Errorf("incomplete WAV output")
		}
		length := int64(binary.LittleEndian.Uint32(chunk[4:]))
		pos, _ := f.Seek(0, io.SeekCurrent)
		if pos+length > info.Size() {
			return 0, fmt.Errorf("truncated WAV chunk")
		}
		switch string(chunk[:4]) {
		case "fmt ":
			var format [16]byte
			if length < 16 {
				return 0, fmt.Errorf("invalid WAV format")
			}
			if _, err := io.ReadFull(f, format[:]); err != nil {
				return 0, err
			}
			if binary.LittleEndian.Uint16(format[:2]) != 1 || binary.LittleEndian.Uint16(format[2:4]) != 2 || binary.LittleEndian.Uint32(format[4:8]) != 44100 || binary.LittleEndian.Uint16(format[12:14]) != 4 || binary.LittleEndian.Uint16(format[14:16]) != 16 {
				return 0, fmt.Errorf("expected stereo 44100 Hz, 16-bit PCM WAV")
			}
			byteRate = binary.LittleEndian.Uint32(format[8:12])
			if byteRate != 176400 {
				return 0, fmt.Errorf("invalid WAV byte rate")
			}
		case "data":
			if byteRate == 0 || length == 0 || length%4 != 0 {
				return 0, fmt.Errorf("empty or invalid PCM data")
			}
			return float64(length) / float64(byteRate), nil
		}
		if _, err := f.Seek(pos+length+length%2, io.SeekStart); err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("too many WAV chunks")
}

type boundedLog struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (l *boundedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	if l.data.Len() < 64<<10 {
		_, _ = l.data.Write(p[:min(len(p), (64<<10)-l.data.Len())])
	}
	return n, nil
}
func (l *boundedLog) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.data.String() }
