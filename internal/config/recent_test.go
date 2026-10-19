//go:build darwin || linux

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestRecentScoresPersistenceOrderAndLimit(t *testing.T) {
	ctx := context.Background()
	store := RecentScores{Directory: filepath.Join(t.TempDir(), "config")}
	if paths, err := store.Load(ctx); err != nil || len(paths) != 0 {
		t.Fatal("new history was not empty", paths, err)
	}
	if _, err := os.Stat(store.Directory); !os.IsNotExist(err) {
		t.Fatal("reading empty history created its directory", err)
	}
	for i := range recentScoreLimit + 4 {
		if err := store.Remember(ctx, filepath.Join(store.Directory, fmt.Sprintf("score %d 雨.ly", i))); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(store.Directory, "score 8 雨.ly")
	if err := store.Remember(ctx, path); err != nil {
		t.Fatal(err)
	}
	paths, err := (RecentScores{Directory: store.Directory}).Load(ctx)
	if err != nil || len(paths) != recentScoreLimit || paths[0] != path || filepath.Base(paths[1]) != "score 23 雨.ly" || filepath.Base(paths[19]) != "score 4 雨.ly" {
		t.Fatal("history did not persist, deduplicate or retain newest scores", paths, err)
	}
	for _, name := range []string{"recent.json", "recent.lock"} {
		info, err := os.Stat(filepath.Join(store.Directory, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("history was not private", name, info, err)
		}
	}
	info, err := os.Stat(store.Directory)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("history directory was not private", info, err)
	}
}

func TestRecentScoresConcurrentLaunchesAndCancellation(t *testing.T) {
	store := RecentScores{Directory: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			if err := store.Remember(ctx, filepath.Join(store.Directory, fmt.Sprintf("%d.ly", i))); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	paths, err := store.Load(ctx)
	if err != nil || len(paths) != 12 {
		t.Fatal("concurrent launches lost scores", paths, err)
	}
	lock, err := os.OpenFile(filepath.Join(store.Directory, "recent.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	bounded, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	if err := store.Remember(bounded, filepath.Join(store.Directory, "blocked.ly")); err != context.DeadlineExceeded {
		t.Fatal("locked history ignored cancellation", err)
	}
	cancelled, stopNow := context.WithCancel(ctx)
	stopNow()
	if _, err := store.Load(cancelled); err != context.Canceled {
		t.Fatal("cancelled load continued", err)
	}
}

func TestRecentScoresInvalidHistory(t *testing.T) {
	store := RecentScores{Directory: t.TempDir()}
	path := filepath.Join(store.Directory, "recent.json")
	good := filepath.Join(store.Directory, "score.ly")
	data, err := json.Marshal(recentFile{Version: 1, Paths: []string{"relative.ly", good, good, "/tmp/wrong.txt", "/tmp/control\x1b.ly"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := store.Load(context.Background())
	if err != nil || !reflect.DeepEqual(paths, []string{good}) {
		t.Fatal("invalid or repeated paths survived loading", paths, err)
	}
	for _, data := range []string{"broken json", `{"version":2,"paths":[]}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := store.Remember(context.Background(), good); err == nil {
			t.Fatal("invalid history was silently replaced")
		}
		if after, _ := os.ReadFile(path); string(after) != data {
			t.Fatal("failed history write destroyed existing data")
		}
	}
}
