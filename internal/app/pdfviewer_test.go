//go:build darwin || linux

package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lyra/internal/input"
)

func TestPDFViewerSnapshotSurvivesRebuildAndReopensAtomically(t *testing.T) {
	root, scores := t.TempDir(), t.TempDir()
	source := filepath.Join(scores, "-étude $(touch injected); chorale.ly")
	userPDF := strings.TrimSuffix(source, ".ly") + ".pdf"
	if err := os.WriteFile(userPDF, []byte("user PDF"), 0600); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(root, "score.pdf")
	if err := os.WriteFile(build, []byte("first generation"), 0600); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan string, 2), make(chan struct{})
	var opens atomic.Int32
	v := newPDFViewer(context.Background(), root, source,
		func() (io.ReadCloser, error) { return os.Open(build) },
		func(ctx context.Context, path string) error {
			opens.Add(1)
			started <- path
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-release:
				return nil
			}
		})
	t.Cleanup(v.Close)
	v.Request()
	var path string
	select {
	case path = <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("opener did not start")
	}
	if filepath.Dir(path) != filepath.Join(root, "viewer") || filepath.Base(path) != filepath.Base(userPDF) {
		t.Fatalf("viewer path = %q", path)
	}
	for range 100 {
		v.Request() // Repeated keys must not queue more launches while busy.
	}
	if err := os.Remove(build); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first generation" {
		t.Fatalf("rebuild removed viewer copy: %q %v", data, err)
	}
	old, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	close(release)
	waitFor(t, "first viewer launch", func() bool { return !v.Snapshot().Busy })
	if v.Snapshot().Err != nil || opens.Load() != 1 {
		t.Fatalf("unexpected launches: %d %+v", opens.Load(), v.Snapshot())
	}
	if err := os.WriteFile(build, []byte("second generation"), 0600); err != nil {
		t.Fatal(err)
	}
	v.Request()
	waitFor(t, "second viewer launch", func() bool { return !v.Snapshot().Busy })
	state := v.Snapshot()
	data, err = os.ReadFile(state.Path)
	if state.Err != nil || err != nil || state.Path != path || string(data) != "second generation" || opens.Load() != 2 {
		t.Fatalf("latest snapshot not opened: %+v %q %v", state, data, err)
	}
	data, err = io.ReadAll(old)
	if err != nil || string(data) != "first generation" {
		t.Fatalf("open descriptor was truncated: %q %v", data, err)
	}
	data, err = os.ReadFile(userPDF)
	if err != nil || string(data) != "user PDF" {
		t.Fatalf("overwrote PDF beside source: %q %v", data, err)
	}
	for _, path := range []string{filepath.Dir(state.Path), state.Path} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("viewer file is not private: %s %v", path, err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("snapshot copies accumulated: %v %v", entries, err)
	}
}

func TestPDFViewerFailuresAndCancellationStayOutOfEditor(t *testing.T) {
	for _, failure := range []string{"missing PDF", "failed copy", "failed opener"} {
		t.Run(failure, func(t *testing.T) {
			errWant := errors.New(failure + "\x1b[2J")
			v := newPDFViewer(context.Background(), t.TempDir(), "score.ly",
				func() (io.ReadCloser, error) {
					if failure == "missing PDF" {
						return nil, errWant
					}
					if failure == "failed copy" {
						return io.NopCloser(failedPDFReader{errWant}), nil
					}
					return io.NopCloser(strings.NewReader("PDF")), nil
				}, func(context.Context, string) error { return errWant })
			t.Cleanup(v.Close)
			m := &model{pdfViewer: v, focus: input.Editor, layout: measure(120, 32)}
			if err := m.dispatch([]input.Event{{Action: input.OpenPDF}}); err != nil || m.focus != input.Editor {
				t.Fatalf("open changed editor focus or failed input: %v", err)
			}
			waitFor(t, failure, func() bool { return !v.Snapshot().Busy })
			state := v.Snapshot()
			m.update(pdfViewerMsg(state))
			if !errors.Is(state.Err, errWant) || m.err != nil || strings.Contains(m.pdfViewerNotice, "\x1b") {
				t.Fatalf("viewer error escaped notice or stopped editor: %+v %q", state, m.pdfViewerNotice)
			}
			lines := strings.Join(m.buildLines(100), "\n")
			if !strings.Contains(lines, failure) || strings.Contains(lines, "\x1b") {
				t.Fatalf("missing/unsafe diagnostic: %q", lines)
			}
			if failure != "failed opener" {
				if entries, _ := os.ReadDir(v.root); len(entries) != 0 {
					t.Fatalf("failed copy left partial files: %v", entries)
				}
			}
		})
	}
	started := make(chan struct{})
	v := newPDFViewer(context.Background(), t.TempDir(), "score.ly",
		func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("PDF")), nil },
		func(ctx context.Context, _ string) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	t.Cleanup(v.Close)
	v.Request()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("opener did not start")
	}
	joined := make(chan struct{})
	go func() { v.Close(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(2 * time.Second):
		t.Fatal("viewer worker did not stop on shutdown")
	}
	v.Request() // Requests after Close cannot restart or block the worker.
}

type failedPDFReader struct{ err error }

func (r failedPDFReader) Read([]byte) (int, error) { return 0, r.err }

func TestPDFOpenerUsesLiteralPathAndNoTerminalInput(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "fake opener")
	script := "#!/bin/sh\n[ \"$#\" -eq 1 ] || exit 2\nIFS= read -r input && exit 3\nprintf '%s' \"$1\" > \"$0.argument\"\necho stdout\necho stderr >&2\n"
	if err := os.WriteFile(tool, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "-étude $(touch injected); one.pdf")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := runPDFOpener(ctx, tool, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tool + ".argument")
	if err != nil || string(data) != path {
		t.Fatalf("path was not one literal argument: %q %v", data, err)
	}
	if err := runPDFOpener(ctx, filepath.Join(dir, "missing viewer"), path); err == nil {
		t.Fatal("missing opener reported success")
	}
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := runPDFOpener(ctx, tool, path); err == nil {
		t.Fatal("failed opener reported success")
	}
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho started > \"$0.started\"\nexec sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- runPDFOpener(runCtx, tool, path) }()
	waitFor(t, "opener process started", func() bool { _, err := os.Stat(tool + ".started"); return err == nil })
	stop()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled opener reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("opener process was not reaped on cancellation")
	}
}
