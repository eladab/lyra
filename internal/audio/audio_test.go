//go:build darwin || linux

package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func waitState(t *testing.T, s *Service, label string, check func(State) bool) State {
	t.Helper()
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	for {
		state := s.Snapshot()
		if check(state) {
			return state
		}
		select {
		case <-s.Changed():
		case <-deadline.C:
			t.Fatalf("%s: %+v", label, state)
			return state
		}
	}
}
func request(t *testing.T, s *Service, action Action) {
	t.Helper()
	if err := s.Request(action); err != nil {
		t.Fatal(err)
	}
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

func realMIDI(t *testing.T) string {
	t.Helper()
	exe := os.Getenv("LYRA_TEST_LILYPOND")
	if exe == "" {
		t.Skip("set LYRA_TEST_LILYPOND for real audio integration")
	}
	dir := t.TempDir()
	source, err := filepath.Abs("../testdata/scale.ly")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--output="+filepath.Join(dir, "score with spaces"), source)
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("LilyPond: %v %s", err, out)
	}
	return filepath.Join(dir, "score with spaces.midi")
}
func realOptions(t *testing.T) Options {
	t.Helper()
	font := os.Getenv("LYRA_TEST_SOUNDFONT")
	if font == "" {
		t.Skip("set LYRA_TEST_SOUNDFONT for real FluidSynth integration")
	}
	for _, tool := range []string{"fluidsynth", "mpv"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatal("missing integration tool: " + tool)
		}
	}
	return Options{SoundFont: font, Output: "null"}
}

func TestRealSynthesisAndPlaybackLifecycle(t *testing.T) {
	opts := realOptions(t)
	midi := realMIDI(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "player-started")
	socketMarker := filepath.Join(dir, "player-socket")
	realMPV, err := exec.LookPath("mpv")
	if err != nil {
		t.Fatal(err)
	}
	spy := filepath.Join(dir, "mpv spy")
	script := "#!/bin/sh\nprintf '%s\\n' \"$$\" > " + shellQuote(marker) + "\nfor arg do\ncase \"$arg\" in --input-ipc-server=*) printf '%s\\n' \"${arg#*=}\" > " + shellQuote(socketMarker) + ";; esac\ndone\nexec " + shellQuote(realMPV) + " \"$@\"\n"
	if err := os.WriteFile(spy, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	opts.MPV = spy
	s, err := NewService(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root := s.root
	source := Source{Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(midi) }}
	s.UpdateSource(source)
	ready := waitState(t, s, "real synthesis ready", func(s State) bool { return s.Status == "Ready" })
	if ready.Duration < 8 || ready.Generation != 1 {
		t.Fatalf("invalid synthesized duration/take: %+v", ready)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("synthesis started player automatically")
	}
	files, err := filepath.Glob(filepath.Join(root, "take-*", "score.wav"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if !hasPCMInk(t, files[0]) {
		t.Fatal("real FluidSynth produced silent PCM")
	}
	for range 3 {
		request(t, s, Quieter)
	}
	waitState(t, s, "volume", func(s State) bool { return s.Volume == 55 })
	request(t, s, Toggle)
	waitState(t, s, "position advances", func(s State) bool { return s.Status == "Playing" && s.Position > .1 })
	request(t, s, Toggle)
	waitState(t, s, "pause", func(s State) bool { return s.Status == "Paused" })
	time.Sleep(300 * time.Millisecond)
	paused := s.Snapshot()
	time.Sleep(300 * time.Millisecond)
	if math.Abs(s.Snapshot().Position-paused.Position) > .1 {
		t.Fatal("paused position kept moving")
	}
	request(t, s, Forward)
	ahead := waitState(t, s, "seek forward while paused", func(s State) bool { return s.Status == "Paused" && s.Position >= paused.Position+4.8 })
	request(t, s, Backward)
	waitState(t, s, "seek backward", func(s State) bool { return s.Position <= ahead.Position-4.8 })
	request(t, s, Restart)
	waitState(t, s, "restart position", func(s State) bool { return s.Position == 0 })
	source.Generation = 2
	s.UpdateSource(source)
	newer := waitState(t, s, "new take ready without interruption", func(s State) bool { return s.ReadyGeneration == 2 })
	if newer.Status != "Paused" || newer.Generation != 1 || newer.Position > .1 {
		t.Fatalf("save interrupted current take: %+v", newer)
	}
	request(t, s, Latest)
	waitState(t, s, "explicit latest take", func(s State) bool { return s.Status == "Playing" && s.Generation == 2 })
	s.UpdateSource(Source{Generation: 3})
	noMIDI := waitState(t, s, "score without MIDI", func(s State) bool { return s.SourceGeneration == 3 && !s.HasMIDI })
	if noMIDI.Status != "Playing" || noMIDI.Generation != 2 {
		t.Fatal("MIDI removal interrupted playing take")
	}
	request(t, s, Stop)
	waitState(t, s, "stop", func(s State) bool { return s.Status == "Stopped" && s.Position == 0 })
	request(t, s, Toggle)
	waitState(t, s, "missing MIDI diagnostic", func(s State) bool { return strings.Contains(s.Error, "No MIDI") })
	source.Generation = 4
	s.UpdateSource(source)
	waitState(t, s, "new playable take", func(s State) bool { return s.ReadyGeneration == 4 })
	request(t, s, Toggle)
	waitState(t, s, "play after stop", func(s State) bool { return s.Status == "Playing" && s.Generation == 4 })
	for i := 0; i < int(math.Ceil(ready.Duration/5))+1; i++ {
		request(t, s, Forward)
	}
	waitState(t, s, "EOF", func(s State) bool { return s.Status == "Ended" && s.Position == s.Duration })
	pidData, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, "unexpected player death", func(s State) bool { return s.Status == "Playback error" })
	request(t, s, Toggle)
	waitState(t, s, "restart player after failure", func(s State) bool { return s.Status == "Playing" && s.Generation == 4 })
	waitState(t, s, "recovered output advances", func(s State) bool { return s.Position > .1 })
	// Freeze the real silent-output player through a second IPC client, without
	// notifying the service. This reproduces a driver that accepts Play but
	// stops advancing, and must produce a useful error rather than false progress.
	socketData, err := os.ReadFile(socketMarker)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", strings.TrimSpace(string(socketData)))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(conn).Encode(map[string]any{"command": []any{"set_property", "pause", true}, "request_id": 1}); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	decoder := json.NewDecoder(conn)
	for {
		var reply message
		if err := decoder.Decode(&reply); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if reply.RequestID == 1 {
			if reply.Error != "success" {
				t.Fatal(reply.Error)
			}
			break
		}
	}
	conn.Close()
	waitState(t, s, "stalled output diagnostic", func(s State) bool { return s.Status == "Playback error" && strings.Contains(s.Error, "not advancing") })
	request(t, s, Toggle)
	waitState(t, s, "retry stalled output", func(s State) bool { return s.Status == "Playing" && s.Position > .1 })
	pidData, err = os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatal("service Close did not reap mpv", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("session WAV/MIDI files were not removed")
	}
}

func TestSynthesisFailuresAndTimeouts(t *testing.T) {
	opts := realOptions(t)
	midi := realMIDI(t)
	badDir := t.TempDir()
	badFont := filepath.Join(badDir, "broken.sf2")
	if err := os.WriteFile(badFont, []byte("RIFF\x04\x00\x00\x00sfbk"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := opts
	broken.SoundFont = badFont
	badResult := render(context.Background(), broken, badDir, Source{Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(midi) }})
	if badResult.err == nil {
		t.Fatal("invalid bank produced false successful synthesis")
	}
	for _, tc := range []struct {
		name, script string
		timeout      time.Duration
	}{{"missing WAV", "#!/bin/sh\nexit 0\n", time.Second}, {"failure", "#!/bin/sh\necho failed >&2\nexit 1\n", time.Second}, {"timeout", "#!/bin/sh\nexec /bin/sleep 30\n", 50 * time.Millisecond}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stub := filepath.Join(dir, "synth")
			if err := os.WriteFile(stub, []byte(tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			custom := opts
			custom.FluidSynth = stub
			custom.Timeout = tc.timeout
			result := render(context.Background(), custom, dir, Source{Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(midi) }})
			if result.err == nil || result.take != nil {
				t.Fatalf("synthesis falsely succeeded: %+v", result)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatal("failed synthesis left temporary artifacts", entries)
			}
		})
	}
}

func hasPCMInk(t *testing.T, path string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for pos := 12; pos+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if pos+8+size > len(data) {
			t.Fatal("truncated test WAV")
		}
		if string(data[pos:pos+4]) == "data" {
			return bytes.Count(data[pos+8:pos+8+size], []byte{0}) < size-1000
		}
		pos += 8 + size + size%2
	}
	return false
}

func TestSoundFontAndMIDIValidation(t *testing.T) {
	if _, err := resolveOptions(Options{FluidSynth: "/missing/fluidsynth"}); err == nil {
		t.Fatal("missing tool accepted")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveOptions(Options{FluidSynth: exe, MPV: exe, SoundFont: filepath.Join(t.TempDir(), "missing.sf2")})
	if err == nil || !strings.Contains(err.Error(), "--soundfont") {
		t.Fatal("missing SoundFont diagnostic", err)
	}
	file := filepath.Join(t.TempDir(), "invalid")
	if err := os.WriteFile(file, []byte("not MIDI"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateMIDI(file); err == nil {
		t.Fatal("invalid MIDI accepted")
	}
	if _, err := waveDuration(file); err == nil {
		t.Fatal("invalid WAV accepted")
	}
	if _, err := resolveOptions(Options{FluidSynth: exe, MPV: exe, SoundFont: file}); err == nil {
		t.Fatal("invalid SoundFont accepted")
	}
}

func TestCancellationRejectsStaleSynthesisAndReapsChild(t *testing.T) {
	opts := realOptions(t)
	midi := realMIDI(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	stub := filepath.Join(dir, "slow-fluid")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\ntouch "+shellQuote(marker)+"\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	opts.FluidSynth = stub
	s, err := NewService(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.UpdateSource(Source{Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(midi) }})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("synth did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	request(t, s, Toggle)
	waitState(t, s, "queued explicit play", func(s State) bool { return s.QueuedPlay })
	s.UpdateSource(Source{Generation: 2})
	state := waitState(t, s, "cancel stale synthesis", func(s State) bool { return s.SourceGeneration == 2 && !s.Preparing })
	if state.ReadyGeneration != 0 || state.QueuedPlay {
		t.Fatal("stale synthesis was published or save queued playback")
	}
	start := time.Now()
	s.Close()
	if time.Since(start) > 2*time.Second {
		t.Fatal("synthesis cancellation did not reap promptly")
	}
}

func TestMacOSAudioOutput(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("LYRA_TEST_MACOS_AUDIO") != "1" {
		t.Skip("set LYRA_TEST_MACOS_AUDIO=1 for a brief low-volume macOS output check")
	}
	opts := realOptions(t)
	opts.Output = "" // Exercise the application's native macOS default.
	diagnostics := filepath.Join(t.TempDir(), "mpv.log")
	playerPath, err := exec.LookPath("mpv")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "mpv-debug")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+shellQuote(playerPath)+" --log-file="+shellQuote(diagnostics)+" \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	opts.MPV = wrapper
	opts, err = resolveOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	midi := realMIDI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := render(ctx, opts, t.TempDir(), Source{Generation: 1, Open: func() (io.ReadCloser, error) { return os.Open(midi) }})
	if result.err != nil {
		t.Fatal(result.err)
	}
	p, err := startPlayer(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if t.Failed() {
			log, _ := os.ReadFile(diagnostics)
			t.Log(string(log))
		}
		if p != nil {
			p.close()
		}
	}()
	if err := p.load(ctx, result.take.wav, 15); err != nil {
		t.Fatal(err)
	}
	if _, err := p.call(ctx, "set_property", "pause", false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := p.call(ctx, "get_property", "time-pos")
		if err != nil {
			t.Fatal(err)
		}
		var pos float64
		_ = json.Unmarshal(data, &pos)
		if pos > .2 {
			break
		}
		if time.Now().After(deadline) {
			var details []string
			for _, prop := range []string{"time-pos", "audio-pts", "audio-out-params", "pause", "paused-for-cache", "current-ao", "core-idle", "eof-reached", "audio-device"} {
				data, err := p.call(ctx, "get_property", prop)
				details = append(details, fmt.Sprintf("%s=%s (%v)", prop, data, err))
			}
			t.Fatalf("macOS audio position did not advance: %s; %s", strings.Join(details, ", "), p.log.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	data, err := p.call(ctx, "get_property", "current-ao")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\"coreaudio\"" {
		t.Fatalf("real CoreAudio output not active: %s", data)
	}
	if _, err := p.call(ctx, "set_property", "pause", true); err != nil {
		t.Fatal(err)
	}
	data, err = p.call(ctx, "get_property", "time-pos")
	if err != nil {
		t.Fatal(err)
	}
	var paused float64
	_ = json.Unmarshal(data, &paused)
	time.Sleep(200 * time.Millisecond)
	data, err = p.call(ctx, "get_property", "time-pos")
	if err != nil {
		t.Fatal(err)
	}
	var position float64
	_ = json.Unmarshal(data, &position)
	if math.Abs(position-paused) > .1 {
		t.Fatal("native audio position advanced while paused")
	}
	if _, err := p.call(ctx, "seek", 5, "absolute+exact"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.call(ctx, "set_property", "pause", false); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for position < 5.2 {
		if time.Now().After(deadline) {
			t.Fatalf("native output did not resume after seek: %f", position)
		}
		data, err = p.call(ctx, "get_property", "time-pos")
		if err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(data, &position)
		time.Sleep(50 * time.Millisecond)
	}
	pid, dir := p.cmd.Process.Pid, p.dir
	p.close()
	p = nil
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatal("mpv child was not reaped", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("IPC socket directory was not removed")
	}
}
