//go:build darwin || linux

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode"
)

type scoreEntry struct {
	name, path string
	dir        bool
}

// Bound directory work and memory even when a folder contains many unrelated
// files. Directory I/O runs as a Bubble Tea command, never on its event loop.
func scoreDirectory(ctx context.Context, path string, hidden bool) ([]scoreEntry, bool, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	var result []scoreEntry
	seen, limited := 0, false
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		entries, err := dir.ReadDir(256)
		for _, entry := range entries {
			seen++
			if seen > 20_000 {
				limited = true
				break
			}
			name := entry.Name()
			if !hidden && strings.HasPrefix(name, ".") {
				continue
			}
			full := filepath.Join(path, name)
			isDir, regular := entry.IsDir(), entry.Type().IsRegular()
			if entry.Type()&os.ModeSymlink != 0 {
				info, statErr := os.Stat(full)
				if statErr != nil {
					continue
				}
				isDir, regular = info.IsDir(), info.Mode().IsRegular()
			}
			if isDir || (regular && strings.EqualFold(filepath.Ext(name), ".ly")) {
				result = append(result, scoreEntry{name: name, path: full, dir: isDir})
			}
		}
		if limited || errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].dir != result[j].dir {
			return result[i].dir
		}
		a, b := strings.ToLower(result[i].name), strings.ToLower(result[j].name)
		if a == b {
			return result[i].name < result[j].name
		}
		return a < b
	})
	parent := filepath.Dir(path)
	if parent != path {
		result = append([]scoreEntry{{name: "..", path: parent, dir: true}}, result...)
	}
	return result, limited, nil
}

func scorePath(value, directory string) (string, error) {
	if value == "" {
		return "", errors.New("Enter a filename or folder path")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 || len(value) > 4096 {
		return "", errors.New("Enter a single path without control characters (up to 4096 bytes)")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, value[2:])
		}
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(directory, value)
	}
	return filepath.Clean(value), nil
}

// Open accepts directories for navigation and readable regular .ly scores.
// Create reserves a new empty file exclusively; existing files and symlinks are
// never overwritten. The selected file belongs to the user and is not temporary.
func chooseScorePath(ctx context.Context, value, directory string, create bool) (path string, isDir bool, err error) {
	path, err = scorePath(value, directory)
	if err != nil {
		return "", false, err
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if create {
		path, err := createScore(ctx, value, directory, "")
		return path, false, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", false, fmt.Errorf("Open score: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return path, true, nil
	}
	if !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".ly") {
		return "", false, errors.New("Choose a regular LilyPond .ly file")
	}
	return path, false, nil
}

// Validating a destination does not create or reserve anything. Creation is
// deferred until the user confirms a template (or the blank option).
func scoreDestination(ctx context.Context, value, directory string) (string, error) {
	path, err := scorePath(value, directory)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.HasSuffix(value, string(filepath.Separator)) {
		return "", errors.New("Include a filename after the folder path")
	}
	if filepath.Ext(path) == "" {
		path += ".ly"
	}
	if !strings.EqualFold(filepath.Ext(path), ".ly") {
		return "", errors.New("A LilyPond score needs a .ly extension")
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || !info.IsDir() {
		return "", errors.New("The destination folder does not exist or cannot be opened")
	}
	if _, err := os.Lstat(path); err == nil {
		return "", errors.New("That file already exists; choose another name or open it")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("Check destination: %w", err)
	}
	return path, nil
}

func createScore(ctx context.Context, value, directory, content string) (string, error) {
	path, err := scoreDestination(ctx, value, directory)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if errors.Is(err, os.ErrExist) {
		return "", errors.New("That file already exists; choose another name or open it")
	}
	if err != nil {
		return "", fmt.Errorf("Create score: %w", err)
	}
	owned, statErr := file.Stat()
	err = ctx.Err()
	if err == nil {
		_, err = io.WriteString(file, content)
	}
	err = errors.Join(statErr, err, file.Close())
	if err != nil {
		// Remove only our failed, newly created file. Preserve a replacement if
		// another process changed the path while the write was in progress.
		if now, e := os.Lstat(path); e == nil && owned != nil && os.SameFile(owned, now) {
			_ = os.Remove(path)
		}
		return "", fmt.Errorf("Write new score: %w", err)
	}
	return path, nil
}

func completeScorePath(ctx context.Context, text, directory string) (string, string, error) {
	if text == "" {
		text = "." + string(filepath.Separator)
	}
	path, err := scorePath(text, directory)
	if err != nil {
		return text, "", err
	}
	parent, prefix := filepath.Dir(path), filepath.Base(path)
	if strings.HasSuffix(text, string(filepath.Separator)) || text == "~" {
		parent, prefix = path, ""
	}
	entries, limited, err := scoreDirectory(ctx, parent, true)
	if err != nil {
		return text, "", err
	}
	var matches []scoreEntry
	for _, entry := range entries {
		if entry.name != ".." && strings.HasPrefix(entry.name, prefix) {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return text, "No matching folders or .ly scores", nil
	}
	common := matches[0].name
	for _, entry := range matches[1:] {
		a, b := []rune(common), []rune(entry.name)
		n := 0
		for n < len(a) && n < len(b) && a[n] == b[n] {
			n++
		}
		common = string(a[:n])
	}
	next := filepath.Join(parent, common)
	if common == "" || (len(matches) == 1 && matches[0].dir) {
		next += string(filepath.Separator)
	}
	notice := ""
	if len(matches) > 1 {
		names := make([]string, 0, min(3, len(matches)))
		for _, entry := range matches[:min(3, len(matches))] {
			names = append(names, entry.name)
		}
		notice = strings.Join(names, " · ")
		if len(matches) > 3 || limited {
			notice += " · …"
		}
	}
	return next, notice, nil
}
