//go:build darwin || linux

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/unix"
)

const recentScoreLimit = 20
const maxRecentBytes = 512 << 10 // Includes JSON escaping of twenty 4096-byte paths.

// RecentScores stores launch targets, newest first. Directory defaults to Lyra's
// platform configuration directory; tests supply a private directory.
type RecentScores struct{ Directory string }

type recentFile struct {
	Version int      `json:"version"`
	Paths   []string `json:"paths"`
}

func (r RecentScores) directory() (string, error) {
	if r.Directory != "" {
		return r.Directory, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lyra"), nil
}

func validRecentPath(path string) bool {
	return filepath.IsAbs(path) && len(path) <= 4096 &&
		strings.EqualFold(filepath.Ext(path), ".ly") &&
		strings.IndexFunc(path, unicode.IsControl) < 0
}

// Load does not stat scores: disconnected volumes and removed files must not
// delay listing history. The chooser validates the selected path before opening.
func (r RecentScores) Load(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := r.directory()
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "recent.json"), os.O_RDONLY|unix.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxRecentBytes {
		return nil, errors.New("recent score history must be a regular file under 512 KiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRecentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRecentBytes {
		return nil, errors.New("recent score history exceeds 512 KiB")
	}
	var history recentFile
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("read recent score history: %w", err)
	}
	if history.Version != 1 {
		return nil, errors.New("unsupported recent score history version")
	}
	paths := make([]string, 0, recentScoreLimit)
	seen := make(map[string]bool)
	for _, path := range history.Paths {
		if !validRecentPath(path) {
			continue
		}
		path = filepath.Clean(path)
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
		if len(paths) == recentScoreLimit {
			break
		}
	}
	return paths, ctx.Err()
}

// Remember merges under a process-shared lock and atomically replaces history,
// so concurrent Lyra launches cannot lose one another's scores or partial writes.
func (r RecentScores) Remember(ctx context.Context, path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !validRecentPath(path) {
		return errors.New("recent scores need a .ly path without control characters (up to 4096 bytes)")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := r.directory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "recent.lock"), os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	paths, err := r.Load(ctx)
	if err != nil {
		return err
	}
	updated := []string{path}
	for _, previous := range paths {
		if previous != path && len(updated) < recentScoreLimit {
			updated = append(updated, previous)
		}
	}
	data, err := json.MarshalIndent(recentFile{Version: 1, Paths: updated}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".recent-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(append(data, '\n'))
	if err := errors.Join(writeErr, tmp.Close(), ctx.Err()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, "recent.json"))
}
