//go:build darwin || linux

package lilypond

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lyra/internal/watcher"
)

type State struct {
	Source           string
	Generation       uint64
	Status           string
	Latest, LastGood *Result
	WatchWarning     string
	WatchedFiles     int
}

// Service serializes builds. Requests invalidate older work immediately, even
// during debounce; only the desired generation may publish artifacts or errors.
type Service struct {
	options                Options
	Root, SaveNotification string
	mu                     sync.Mutex
	state                  State
	wanted, immediate      bool
	awaitingSave           bool
	closed                 bool
	sourceRevision         uint64
	requests, changed      chan struct{}
	watchRequests          chan struct{}
	done, watchDone        chan struct{}
	cancel                 context.CancelFunc
	closeOnce              sync.Once
}

func NewService(parent context.Context, opts Options) (*Service, error) {
	var err error
	opts.Source, err = filepath.Abs(opts.Source)
	if err != nil {
		return nil, err
	}
	opts.IncludeDirs = append([]string(nil), opts.IncludeDirs...)
	for i, dir := range opts.IncludeDirs {
		opts.IncludeDirs[i], err = filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
	}
	root, err := os.MkdirTemp("", "lyra-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{options: opts, Root: root, SaveNotification: filepath.Join(root, "vim-save"), awaitingSave: opts.WaitForSave,
		requests: make(chan struct{}, 1), changed: make(chan struct{}, 1), done: make(chan struct{}), watchDone: make(chan struct{}), cancel: cancel,
		watchRequests: make(chan struct{}, 1), state: State{Source: opts.Source, Status: "Watching launch score"}}
	if opts.WaitForSave {
		s.state.Status = "New sketch"
	}
	go s.run(ctx)
	go s.watch(ctx)
	return s, nil
}

// watch owns watcher replacement, so retargeting never waits for filesystem I/O
// on the UI goroutine. Revision checks reject callbacks from the former score,
// including callbacks already in flight when its watcher is canceled.
func (s *Service) watch(ctx context.Context) {
	defer close(s.watchDone)
	var revision uint64
	var stop context.CancelFunc
	var done chan struct{}
	defer func() {
		if stop != nil {
			stop()
			<-done
		}
	}()
	for {
		s.mu.Lock()
		source, desired := s.state.Source, s.sourceRevision
		s.mu.Unlock()
		if stop == nil || revision != desired {
			if stop != nil {
				stop()
				<-done
			}
			watchCtx, cancel := context.WithCancel(ctx)
			stop = cancel
			done = make(chan struct{})
			revision = desired
			go func(revision uint64, done chan struct{}) {
				defer close(done)
				defer cancel()
				watcher.Run(watchCtx, source, s.SaveNotification, s.options.IncludeDirs, func(event watcher.Event) {
					s.mu.Lock()
					defer s.mu.Unlock()
					if s.closed || revision != s.sourceRevision {
						return
					}
					s.state.WatchWarning, s.state.WatchedFiles = event.Warning, event.Files
					if event.Changed {
						s.requestLocked(false)
					}
					s.notify()
				})
			}(revision, done)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.watchRequests:
		}
	}
}

func (s *Service) notify() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}

func (s *Service) Changed() <-chan struct{} { return s.changed }
func (s *Service) Done() <-chan struct{}    { return s.done }
func (s *Service) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// OpenPDF pins the current PDF by opening it under the publication lock. Unix
// keeps that descriptor valid even when the next build removes its directory.
// Preview copies it into its own bounded, session-local document off the UI.
func (s *Service) OpenPDF(generation uint64) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.state.LastGood
	if r == nil || r.Generation != generation || len(r.PDFs) == 0 {
		return nil, fmt.Errorf("PDF generation was superseded")
	}
	return os.Open(r.PDFs[0])
}

// OpenLatestPDF pins the latest successful PDF for an explicit external open.
// Selecting and opening it under the same lock prevents a rebuild from removing
// the generation between those operations. The caller owns the descriptor.
func (s *Service) OpenLatestPDF() (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.state.LastGood
	if s.closed || r == nil {
		return nil, fmt.Errorf("no successful build yet; save with :w first")
	}
	if len(r.PDFs) == 0 {
		return nil, fmt.Errorf("last successful build has no PDF; add a layout block")
	}
	return os.Open(r.PDFs[0])
}

// OpenMIDI gives synthesis the same descriptor ownership as PDF preview.
func (s *Service) OpenMIDI(generation uint64) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.state.LastGood
	if r == nil || r.Generation != generation || len(r.MIDIs) == 0 {
		return nil, fmt.Errorf("MIDI generation was superseded")
	}
	return os.Open(r.MIDIs[0])
}

func (s *Service) Request(immediate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.requestLocked(immediate)
	}
}

func (s *Service) requestLocked(immediate bool) {
	if s.awaitingSave {
		return
	}
	s.state.Generation++
	s.state.Status, s.state.Latest = "Queued", nil
	s.wanted, s.immediate = true, immediate
	s.notify()
	select {
	case s.requests <- struct{}{}:
	default:
	}
}

// Retarget adopts the successful first save of a temporary draft. Named-file
// launches keep their launch target; callers must only invoke this for that
// explicit draft adoption. The target and generation change together, while
// jobs and directory watches are canceled/replaced off the caller's goroutine.
// LastGood remains available until the new target produces a valid result.
func (s *Service) Retarget(path string) error {
	if path == "" {
		return fmt.Errorf("score path is empty")
	}
	source, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve score path: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if source == s.state.Source && !s.awaitingSave {
		return nil
	}
	s.awaitingSave = false
	s.state.Source = source
	s.state.WatchWarning, s.state.WatchedFiles = "", 0
	s.sourceRevision++
	s.requestLocked(true)
	select {
	case s.watchRequests <- struct{}{}:
	default:
	}
	return nil
}

func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.state.Generation++
	s.state.Status = "Canceled"
	s.wanted = false
	s.notify()
	select {
	case s.requests <- struct{}{}:
	default:
	}
}

func (s *Service) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.cancel()
		<-s.done
		<-s.watchDone
		_ = os.RemoveAll(s.Root)
	})
}

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	complete := make(chan Result, 1)
	var buildCancel context.CancelFunc
	var timer *time.Timer
	var due <-chan time.Time
	var scheduled, active uint64
	running, ready := false, false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			if running {
				buildCancel()
				<-complete
			}
			return
		case <-s.requests:
			s.mu.Lock()
			wanted, immediate, generation := s.wanted, s.immediate, s.state.Generation
			s.mu.Unlock()
			if running && wanted && generation == active {
				continue // a coalesced notification for work already started
			}
			if running {
				buildCancel()
			}
			if timer != nil {
				timer.Stop()
			}
			due, ready = nil, false
			scheduled = generation
			if wanted {
				delay := s.options.Debounce
				if delay <= 0 {
					delay = 200 * time.Millisecond
				}
				if immediate {
					delay = 0
				}
				timer = time.NewTimer(delay)
				due = timer.C
			}
		case <-due:
			due, ready = nil, true
		case result := <-complete:
			buildCancel()
			running = false
			s.mu.Lock()
			var remove string
			if result.Generation == s.state.Generation && s.wanted {
				s.state.Latest = &result
				s.wanted = false
				if result.Err == nil {
					s.state.Status = "Ready"
					if s.state.LastGood != nil {
						remove = s.state.LastGood.Dir
					}
					s.state.LastGood = &result
				} else {
					s.state.Status, remove = "Failed", result.Dir
				}
				s.notify()
			} else {
				remove = result.Dir
			}
			s.mu.Unlock()
			if remove != "" {
				_ = os.RemoveAll(remove)
			}
		}
		if ready && !running {
			s.mu.Lock()
			generation := scheduled
			wanted := s.wanted && generation == s.state.Generation
			opts := s.options
			opts.Source = s.state.Source
			if wanted {
				s.state.Status = "Building"
				s.notify()
			}
			s.mu.Unlock()
			ready = false
			if wanted {
				buildCtx, cancel := context.WithCancel(ctx)
				buildCancel = cancel
				active = generation
				running = true
				go func() {
					defer cancel()
					complete <- compile(buildCtx, opts, s.Root, generation)
				}()
			}
		}
	}
}
