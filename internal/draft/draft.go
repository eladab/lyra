// Package draft suggests musical score names and provides private temporary
// score utilities. The startup chooser uses SuggestedName without creating a
// temporary score; callers of Create own its conservative cleanup policy.
package draft

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

var moods = [...]string{
	"amber", "copper", "dusk", "moonlit", "paper", "silver", "starlit", "velvet",
}

var forms = [...]string{
	"arabesque", "cadenza", "caprice", "nocturne", "prelude", "rhapsody", "serenade", "waltz",
}

// Draft is an empty score in a private directory. Its exported paths are for
// opening and reporting the score; Discard retains its original paths privately.
type Draft struct {
	Path string
	Dir  string

	path    string
	dir     string
	dirInfo os.FileInfo
}

// Create makes a score such as /tmp/lyra-sketch-1234/starlit-cadenza.ly.
// The private, uniquely created directory prevents collisions even when two
// sessions choose the same readable score name.
func Create() (*Draft, error) {
	return createIn("/tmp")
}

// SuggestedName offers a musical filename without creating any files.
func SuggestedName() (string, error) {
	mood, err := rand.Int(rand.Reader, big.NewInt(int64(len(moods))))
	if err != nil {
		return "", fmt.Errorf("choose score name: %w", err)
	}
	form, err := rand.Int(rand.Reader, big.NewInt(int64(len(forms))))
	if err != nil {
		return "", fmt.Errorf("choose score name: %w", err)
	}
	return moods[mood.Int64()] + "-" + forms[form.Int64()] + ".ly", nil
}

func createIn(root string) (*Draft, error) {
	dir, err := os.MkdirTemp(root, "lyra-sketch-")
	if err != nil {
		return nil, fmt.Errorf("create draft directory: %w", err)
	}
	// Until creation succeeds, the directory has no user data or recovery files.
	cleanup := func() { _ = os.Remove(dir) }
	info, err := os.Lstat(dir)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("inspect draft directory: %w", err)
	}
	name, err := SuggestedName()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("choose draft name: %w", err)
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create draft score: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		cleanup()
		return nil, fmt.Errorf("close draft score: %w", err)
	}
	return &Draft{Path: path, Dir: dir, path: path, dir: dir, dirInfo: info}, nil
}

// Discard removes only this score and its empty original directory. The caller
// must first establish that the draft is safely saved elsewhere or explicitly
// abandoned. Any other files, including Vim recovery files, are preserved.
// Repeated calls are harmless. A zero-value Draft is also safe to discard.
func (d *Draft) Discard() error {
	if d == nil || d.dir == "" {
		return nil
	}
	info, err := os.Lstat(d.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect draft directory: %w", err)
	}
	if !info.IsDir() || !os.SameFile(d.dirInfo, info) {
		return errors.New("draft directory was replaced; preserving it")
	}
	info, err = os.Lstat(d.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect draft score: %w", err)
	}
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("draft score is no longer a regular file; preserving it")
		}
		if err := os.Remove(d.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove draft score: %w", err)
		}
	}
	if err := os.Remove(d.dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove empty draft directory: %w", err)
	}
	return nil
}
