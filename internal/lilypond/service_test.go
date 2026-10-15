//go:build darwin || linux

package lilypond

import (
	"context"
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
