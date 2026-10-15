//go:build darwin || linux

package lilypond

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func until(t *testing.T, description string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout: " + description)
}

func TestServiceSavesStaleResultsFailureAndCleanup(t *testing.T) {
	file := filepath.Join(t.TempDir(), "score.ly")
	writeScore(t, file, "good")
	s, err := NewService(context.Background(), Options{Source: file, Executable: fakeCompiler(t), Debounce: 40 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	until(t, "initial successful build", func() bool { return s.Snapshot().Status == "Ready" })
	good := s.Snapshot().LastGood
	if good == nil {
		t.Fatal("missing good generation")
	}
	writeScore(t, file, "slow")
	until(t, "slow build starts", func() bool { _, err := os.Stat(file + ".started"); return err == nil })
	for i := range 8 {
		writeScore(t, file, "new score "+strings.Repeat("x", i))
		s.Request(false)
	}
	until(t, "only newest score published", func() bool {
		state := s.Snapshot()
		if state.Status != "Ready" || state.LastGood == nil || state.LastGood.Generation <= good.Generation {
			return false
		}
		data, _ := os.ReadFile(state.LastGood.PDFs[0])
		return strings.Contains(string(data), "new score xxxxxxx")
	})
	good = s.Snapshot().LastGood
	writeScore(t, file, "fail")
	until(t, "new error visible", func() bool { return s.Snapshot().Status == "Failed" })
	state := s.Snapshot()
	if state.LastGood != good || len(state.Latest.Diagnostics) != 1 {
		t.Fatal("failure replaced good artifacts or lost diagnostics")
	}
	if _, err := os.Stat(good.PDFs[0]); err != nil {
		t.Fatal("last successful PDF was removed")
	}
	writeScore(t, file, "slow")
	_ = os.Remove(file + ".started")
	until(t, "cancelable build starts", func() bool { _, err := os.Stat(file + ".started"); return err == nil })
	s.Cancel()
	until(t, "canceled state", func() bool { return s.Snapshot().Status == "Canceled" })
	if s.Snapshot().LastGood != good {
		t.Fatal("cancel replaced good artifacts")
	}
	start := time.Now()
	s.Close()
	if time.Since(start) > 2*time.Second {
		t.Fatal("compiler process did not stop promptly")
	}
	if _, err := os.Stat(s.Root); !os.IsNotExist(err) {
		t.Fatal("build directory leaked")
	}
}

func TestIncludeAndAtomicRenameRebuild(t *testing.T) {
	dir := t.TempDir()
	file, include := filepath.Join(dir, "score.ly"), filepath.Join(dir, "part.ily")
	writeScore(t, file, "good\n\\include \"part.ily\"\n")
	writeScore(t, include, "notes")
	s, err := NewService(context.Background(), Options{Source: file, Executable: fakeCompiler(t), Debounce: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	until(t, "initial build", func() bool { return s.Snapshot().Status == "Ready" })
	for _, path := range []string{file, include} {
		previous := s.Snapshot().Generation
		data, _ := os.ReadFile(path)
		writeScore(t, path+".replacement", string(data)+"\n")
		if err := os.Rename(path+".replacement", path); err != nil {
			t.Fatal(err)
		}
		until(t, "atomic rename rebuild", func() bool { state := s.Snapshot(); return state.Status == "Ready" && state.Generation > previous })
	}
}

func TestDraftWaitsForSaveAndWatchesAdoptedScore(t *testing.T) {
	draft := filepath.Join(t.TempDir(), "moonlit-minuet.ly")
	writeScore(t, draft, "draft")
	dir := t.TempDir()
	source, include := filepath.Join(dir, "first étude with spaces.ly"), filepath.Join(dir, "part.ily")
	writeScore(t, source, "adopted score\n\\include \"part.ily\"\n")
	writeScore(t, include, "notes")
	includeDir := t.TempDir()
	s, err := NewService(context.Background(), Options{
		Source: draft, Executable: fakeCompiler(t), Debounce: 25 * time.Millisecond,
		IncludeDirs: []string{includeDir}, WaitForSave: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	until(t, "draft watches established", func() bool { return s.Snapshot().WatchedFiles == 2 })
	s.Request(true)
	writeScore(t, draft, "draft changed")
	writeScore(t, s.SaveNotification, "saved draft")
	time.Sleep(150 * time.Millisecond)
	state := s.Snapshot()
	if state.Status != "New sketch" || state.Generation != 0 || state.Source != draft || state.Latest != nil {
		t.Fatalf("draft compiled before adoption: %+v", state)
	}
	if _, err := os.Stat(draft + ".args"); !os.IsNotExist(err) {
		t.Fatal("compiler started for draft")
	}
	if err := s.Retarget(source); err != nil {
		t.Fatal(err)
	}
	until(t, "adopted source built and watches established", func() bool {
		state := s.Snapshot()
		return state.Status == "Ready" && state.Source == source && state.LastGood != nil && state.WatchedFiles == 3
	})
	state = s.Snapshot()
	if state.Generation == 0 || !strings.Contains(readTestFile(t, state.LastGood.PDFs[0]), "adopted score") {
		t.Fatal("adopted source did not produce its own generation")
	}
	if cwd := strings.TrimSpace(readTestFile(t, source+".cwd")); filepath.Clean(cwd) != filepath.Clean(dir) {
		// macOS resolves /var through /private/var for the compiler's cwd.
		got, _ := filepath.EvalSymlinks(cwd)
		want, _ := filepath.EvalSymlinks(dir)
		if got != want {
			t.Fatalf("adopted compiler cwd = %q, want %q", cwd, dir)
		}
	}
	if args := readTestFile(t, source+".args"); !strings.Contains(args, "--include="+includeDir+"\n") || !strings.HasSuffix(args, source+"\n") {
		t.Fatalf("retarget changed include or source arguments: %q", args)
	}
	previous := state.Generation
	writeScore(t, include+".replacement", "new notes")
	if err := os.Rename(include+".replacement", include); err != nil {
		t.Fatal(err)
	}
	until(t, "adopted include save rebuilt", func() bool {
		state := s.Snapshot()
		return state.Status == "Ready" && state.Generation > previous
	})
	previous = s.Snapshot().Generation
	writeScore(t, source+".replacement", "adopted score updated\n\\include \"part.ily\"\n")
	if err := os.Rename(source+".replacement", source); err != nil {
		t.Fatal(err)
	}
	until(t, "adopted atomic source save rebuilt", func() bool {
		state := s.Snapshot()
		return state.Status == "Ready" && state.Generation > previous && strings.Contains(readTestFile(t, state.LastGood.PDFs[0]), "adopted score updated")
	})
	previous = s.Snapshot().Generation
	writeScore(t, draft, "former draft changed")
	time.Sleep(1200 * time.Millisecond) // includes the polling fallback interval
	if state := s.Snapshot(); state.Generation != previous || state.Source != source {
		t.Fatalf("former source still triggered compilation: %+v", state)
	}
}

func TestRetargetCancelsStaleBuildAndRetainsGoodResult(t *testing.T) {
	oldSource := filepath.Join(t.TempDir(), "draft.ly")
	writeScore(t, oldSource, "initial good")
	s, err := NewService(context.Background(), Options{Source: oldSource, Executable: fakeCompiler(t), Debounce: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	until(t, "old good build", func() bool { return s.Snapshot().Status == "Ready" })
	good := s.Snapshot().LastGood
	writeScore(t, oldSource, "slow old source")
	until(t, "old source compiling", func() bool { _, err := os.Stat(oldSource + ".started"); return err == nil })
	stale := s.Snapshot().Generation
	newSource := filepath.Join(t.TempDir(), "new name.ly")
	writeScore(t, newSource, "fail")
	start := time.Now()
	if err := s.Retarget(newSource); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatal("retarget waited for compiler or filesystem watcher shutdown")
	}
	if state := s.Snapshot(); state.Source != newSource || state.Generation <= stale || state.LastGood != good {
		t.Fatalf("retarget did not atomically invalidate the old source: %+v", state)
	}
	until(t, "adopted source failure", func() bool { return s.Snapshot().Status == "Failed" })
	state := s.Snapshot()
	if state.LastGood != good || state.Latest == nil || state.Latest.Generation <= stale || len(state.Latest.Diagnostics) != 1 || state.Latest.Diagnostics[0].File != newSource {
		t.Fatalf("retarget failure published stale result or discarded good take: %+v", state)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("old compiler was not canceled promptly")
	}
	if _, err := os.Stat(good.PDFs[0]); err != nil {
		t.Fatal("retarget failure removed last good PDF")
	}
	if f, err := s.OpenMIDI(good.Generation); err != nil {
		t.Fatal("retarget failure invalidated last good MIDI:", err)
	} else {
		_ = f.Close()
	}
	oldGeneration := state.Generation
	writeScore(t, newSource, "new good")
	until(t, "adopted source recovery", func() bool {
		state := s.Snapshot()
		return state.Status == "Ready" && state.LastGood.Generation > oldGeneration && strings.Contains(readTestFile(t, state.LastGood.PDFs[0]), "new good")
	})
	if _, err := os.Stat(good.Dir); !os.IsNotExist(err) {
		t.Fatal("superseded good build was not reclaimed")
	}
	s.Close()
	if err := s.Retarget(oldSource); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed retarget error = %v", err)
	}
	if _, err := os.Stat(s.Root); !os.IsNotExist(err) {
		t.Fatal("retarget leaked build artifacts")
	}
}

func TestDraftCanAdoptItsExistingPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "sketch.ly")
	writeScore(t, source, "good")
	s, err := NewService(context.Background(), Options{Source: source, Executable: fakeCompiler(t), WaitForSave: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Retarget(""); err == nil {
		t.Fatal("empty score path was accepted")
	}
	if err := s.Retarget(source); err != nil {
		t.Fatal(err)
	}
	until(t, "same-path adoption built", func() bool { return s.Snapshot().Status == "Ready" })
	previous := s.Snapshot().Generation
	if err := s.Retarget(source); err != nil || s.Snapshot().Generation != previous {
		t.Fatal("duplicate adoption rebuilt unchanged source")
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
