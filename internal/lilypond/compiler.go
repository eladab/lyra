//go:build darwin || linux

// Package lilypond runs engraving jobs without sharing output with the UI.
package lilypond

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Options struct {
	Source, Executable string
	IncludeDirs        []string
	Timeout, Debounce  time.Duration
	// WaitForSave keeps a temporary draft idle until Retarget adopts its first
	// successful save. Its save notification and directory watcher still exist.
	WaitForSave bool
}

type Result struct {
	Generation  uint64
	Dir         string
	PDFs, MIDIs []string
	Log         string
	Truncated   bool
	Diagnostics []Diagnostic
	Duration    time.Duration
	Err         error
}

// cappedLog keeps draining a noisy child even after the retained log is full.
type cappedLog struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (l *cappedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := min(len(p), 256*1024-len(l.data))
	l.data = append(l.data, p[:n]...)
	l.truncated = l.truncated || n < len(p)
	return len(p), nil
}

func compile(ctx context.Context, opts Options, root string, generation uint64) (r Result) {
	start := time.Now()
	r.Generation = generation
	defer func() { r.Duration = time.Since(start) }()
	var err error
	opts.Source, err = filepath.Abs(opts.Source)
	if err != nil {
		r.Err = err
		return
	}
	if _, err := os.Stat(opts.Source); err != nil {
		r.Err = fmt.Errorf("read launch score (save it in Vim first): %w", err)
		return
	}
	executable := opts.Executable
	if executable == "" {
		executable = "lilypond"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		r.Err = fmt.Errorf("LilyPond not found: install it or use --lilypond /path/to/lilypond: %w", err)
		return
	}
	path, err = filepath.Abs(path)
	if err != nil {
		r.Err = err
		return
	}
	r.Dir, err = os.MkdirTemp(root, fmt.Sprintf("build-%d-", generation))
	if err != nil {
		r.Err = fmt.Errorf("create build directory: %w", err)
		return
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"--pdf", "--loglevel=INFO", "-drelative-includes", "--output=" + filepath.Join(r.Dir, "score")}
	for _, dir := range opts.IncludeDirs {
		args = append(args, "--include="+dir)
	}
	// LilyPond 2.26 treats -- as a filename. An absolute source path cannot be
	// mistaken for an option, including when its basename begins with a dash.
	args = append(args, opts.Source)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = filepath.Dir(opts.Source)
	// Diagnostic severities are parsed in English; keep the rest of the user's
	// LilyPond environment (including data/runtime paths) intact.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 500 * time.Millisecond
	log := &cappedLog{}
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err == nil {
		err = cmd.Wait() // one owner reaps the compiler and joins its output readers
		if err != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	r.Log, r.Truncated = string(log.data), log.truncated
	r.Diagnostics = ParseDiagnostics(r.Log, cmd.Dir)
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.Err = fmt.Errorf("LilyPond timed out after %s", timeout)
		} else {
			r.Err = context.Canceled
		}
		return
	}
	if err != nil {
		r.Err = fmt.Errorf("LilyPond failed: %w", err)
		return
	}
	err = filepath.WalkDir(r.Dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		ext := strings.ToLower(filepath.Ext(path))
		if entry.IsDir() || (ext != ".pdf" && ext != ".midi" && ext != ".mid") {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("invalid output file %s", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		var magic [5]byte
		n, _ := io.ReadFull(file, magic[:])
		_ = file.Close()
		if ext == ".pdf" && n == 5 && string(magic[:]) == "%PDF-" {
			r.PDFs = append(r.PDFs, path)
		} else if ext != ".pdf" && n >= 4 && string(magic[:4]) == "MThd" {
			r.MIDIs = append(r.MIDIs, path)
		} else {
			return fmt.Errorf("invalid %s output: %s", ext, filepath.Base(path))
		}
		return nil
	})
	if err != nil {
		r.Err = fmt.Errorf("inspect LilyPond output: %w", err)
	} else if len(r.PDFs)+len(r.MIDIs) == 0 {
		r.Err = errors.New("LilyPond produced no PDF or MIDI; add a score with a layout or MIDI block")
	}
	return
}
