package draft

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestCreatePrivateNamedScore(t *testing.T) {
	root := t.TempDir()
	d, err := createIn(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(d.Dir) != root || filepath.Dir(d.Path) != d.Dir || !strings.HasPrefix(filepath.Base(d.Dir), "lyra-sketch-") {
		t.Fatalf("unexpected draft paths: %+v", d)
	}
	if !regexp.MustCompile(`^[a-z]+-[a-z]+\.ly$`).MatchString(filepath.Base(d.Path)) {
		t.Fatalf("unexpected score name: %s", d.Path)
	}
	for path, mode := range map[string]os.FileMode{d.Dir: 0700, d.Path: 0600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("%s permissions: got %o, want %o", path, info.Mode().Perm(), mode)
		}
	}
	contents, err := os.ReadFile(d.Path)
	if err != nil || len(contents) != 0 {
		t.Fatalf("score is not empty: %q, %v", contents, err)
	}
	if err := d.Discard(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("draft directory remains: %v", err)
	}
	if err := d.Discard(); err != nil {
		t.Fatalf("repeated discard: %v", err)
	}
}

func TestConcurrentDraftsDoNotCollide(t *testing.T) {
	root := t.TempDir()
	const count = 80
	drafts := make(chan *Draft, count)
	errs := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() {
			d, err := createIn(root)
			if err != nil {
				errs <- err
				return
			}
			drafts <- d
		})
	}
	workers.Wait()
	close(drafts)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	seen := map[string]bool{}
	for d := range drafts {
		if seen[d.Path] || seen[d.Dir] {
			t.Errorf("draft collision: %+v", d)
		}
		seen[d.Path], seen[d.Dir] = true, true
		if err := os.WriteFile(d.Path, []byte(d.Path), 0600); err != nil {
			t.Error(err)
		}
	}
	if len(seen) != count*2 {
		t.Fatalf("got %d unique paths, want %d", len(seen), count*2)
	}
	for path := range seen {
		if filepath.Ext(path) != ".ly" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != path {
			t.Errorf("draft was overwritten: %s: %q, %v", path, data, err)
		}
	}
}

func TestDiscardPreservesOtherFiles(t *testing.T) {
	d, err := createIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	recovery := filepath.Join(d.Dir, ".score.ly.swp")
	if err := os.WriteFile(recovery, []byte("recovery"), 0600); err != nil {
		t.Fatal(err)
	}
	// Exported paths cannot redirect cleanup to unrelated files.
	d.Path, d.Dir = recovery, filepath.Dir(d.Dir)
	if err := d.Discard(); err == nil {
		t.Fatal("expected a nonempty-directory error")
	}
	if data, err := os.ReadFile(recovery); err != nil || string(data) != "recovery" {
		t.Fatalf("recovery file changed: %q, %v", data, err)
	}
	if _, err := os.Stat(d.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("own score remains: %v", err)
	}
}

func TestDiscardPreservesReplacementDirectoryAndSymlink(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		d, err := createIn(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		moved := d.Dir + "-moved"
		if err := os.Rename(d.Dir, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(d.Dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d.Path, []byte("replacement"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := d.Discard(); err == nil {
			t.Fatal("expected a replaced-directory error")
		}
		for path, expected := range map[string]string{d.Path: "replacement", filepath.Join(moved, filepath.Base(d.Path)): ""} {
			if data, err := os.ReadFile(path); err != nil || string(data) != expected {
				t.Fatalf("file changed: %s: %q, %v", path, data, err)
			}
		}
	})
	t.Run("symlink", func(t *testing.T) {
		d, err := createIn(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "score.ly")
		if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(d.Path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, d.Path); err != nil {
			t.Fatal(err)
		}
		if err := d.Discard(); err == nil {
			t.Fatal("expected a nonregular-score error")
		}
		if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
			t.Fatalf("target changed: %q, %v", data, err)
		}
		if info, err := os.Lstat(d.Path); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("symlink removed: %v", err)
		}
	})
}

func TestCreateMissingRootAndDiscardZeroValue(t *testing.T) {
	if _, err := createIn(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a nonexistent root")
	}
	if err := (*Draft)(nil).Discard(); err != nil {
		t.Fatal(err)
	}
	if err := (&Draft{}).Discard(); err != nil {
		t.Fatal(err)
	}
}
