//go:build darwin || linux

package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

type pdfViewerState struct {
	Busy bool
	Path string
	Err  error
}

type pdfViewerMsg pdfViewerState

// pdfViewer owns one bounded worker and one session-local PDF snapshot. Builds
// never modify the snapshot; only another explicit open replaces it atomically.
// Close joins the worker before the enclosing build directory is removed.
type pdfViewer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	root      string
	name      string
	read      func() (io.ReadCloser, error)
	launch    func(context.Context, string) error
	requests  chan struct{}
	changed   chan struct{}
	done      chan struct{}
	mu        sync.Mutex
	state     pdfViewerState
	closed    bool
	closeOnce sync.Once
}

func newPDFViewer(parent context.Context, root, source string, read func() (io.ReadCloser, error), launch func(context.Context, string) error) *pdfViewer {
	ctx, cancel := context.WithCancel(parent)
	name := filepath.Base(source)
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ".pdf"
	v := &pdfViewer{ctx: ctx, cancel: cancel, root: filepath.Join(root, "viewer"), name: name, read: read, launch: launch,
		requests: make(chan struct{}, 1), changed: make(chan struct{}, 1), done: make(chan struct{})}
	go v.run()
	return v
}

func (v *pdfViewer) Snapshot() pdfViewerState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state
}

func (v *pdfViewer) Changed() <-chan struct{} { return v.changed }
func (v *pdfViewer) Done() <-chan struct{}    { return v.done }

func (v *pdfViewer) notify() {
	select {
	case v.changed <- struct{}{}:
	default:
	}
}

func (v *pdfViewer) Request() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed || v.state.Busy || v.ctx.Err() != nil {
		return
	}
	v.state.Busy, v.state.Err = true, nil
	v.requests <- struct{}{}
	v.notify()
}

func (v *pdfViewer) Close() {
	v.closeOnce.Do(func() {
		v.mu.Lock()
		v.closed = true
		v.mu.Unlock()
		v.cancel()
		<-v.done
	})
}

func (v *pdfViewer) run() {
	defer close(v.done)
	for {
		select {
		case <-v.ctx.Done():
			return
		case <-v.requests:
			ctx, cancel := context.WithTimeout(v.ctx, 10*time.Second)
			path, err := v.open(ctx)
			cancel()
			v.mu.Lock()
			v.state.Busy, v.state.Err = false, err
			if path != "" {
				v.state.Path = path
			}
			v.notify()
			v.mu.Unlock()
		}
	}
}

func (v *pdfViewer) open(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	pdf, err := v.read()
	if err != nil {
		return "", err
	}
	defer pdf.Close()
	if err := os.MkdirAll(v.root, 0700); err != nil {
		return "", fmt.Errorf("prepare PDF for viewer: %w", err)
	}
	file, err := os.CreateTemp(v.root, ".incoming-")
	if err != nil {
		return "", fmt.Errorf("prepare PDF for viewer: %w", err)
	}
	defer os.Remove(file.Name())
	_, copyErr := io.Copy(file, pdfViewerReader{ctx, pdf})
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("copy PDF for viewer: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("copy PDF for viewer: %w", closeErr)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	path := filepath.Join(v.root, v.name)
	if err := os.Rename(file.Name(), path); err != nil {
		return "", fmt.Errorf("prepare PDF for viewer: %w", err)
	}
	if err := v.launch(ctx, path); err != nil {
		return path, fmt.Errorf("launch default PDF viewer: %w", err)
	}
	return path, nil
}

type pdfViewerReader struct {
	ctx context.Context
	r   io.Reader
}

func (r pdfViewerReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func launchPDFViewer(ctx context.Context, path string) error {
	tool := "xdg-open"
	if runtime.GOOS == "darwin" {
		tool = "/usr/bin/open"
	}
	return runPDFOpener(ctx, tool, path)
}

func runPDFOpener(ctx context.Context, tool, path string) error {
	// Absolute paths cannot be interpreted as options. Neither the opener nor
	// a fallback viewer may read terminal input or write over Bubble Tea output.
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, tool, path)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.WaitDelay = 500 * time.Millisecond
	return cmd.Run()
}

func (m *model) waitPDFViewer() tea.Cmd {
	if m.pdfViewer == nil {
		return nil
	}
	v := m.pdfViewer
	return func() tea.Msg {
		select {
		case <-v.Changed():
			return pdfViewerMsg(v.Snapshot())
		case <-v.Done():
			return nil
		}
	}
}

func (m *model) openPDFViewer() {
	if m.pdfViewer == nil {
		m.pdfViewerState.Err = fmt.Errorf("PDF builds are unavailable")
		m.pdfViewerNotice = "Open PDF: builds unavailable; Ctrl+\\ d for details"
		return
	}
	m.pdfViewer.Request()
	m.pdfViewerState = m.pdfViewer.Snapshot()
	m.pdfViewerNotice = "Opening PDF in default viewer..."
}
