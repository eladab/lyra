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
	Generation       uint64
	Status           string
	Latest, LastGood *Result
	WatchWarning     string
	WatchedFiles     int
}

// Service serializes builds. Requests invalidate older work immediately, even
// during debounce; only the desired generation may publish artifacts or errors.
type Service struct {
	Options
	Root, SaveNotification string
	mu                     sync.Mutex
	state                  State
	wanted, immediate      bool
	requests, changed      chan struct{}
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
	s := &Service{Options: opts, Root: root, SaveNotification: filepath.Join(root, "vim-save"),
		requests: make(chan struct{}, 1), changed: make(chan struct{}, 1), done: make(chan struct{}), watchDone: make(chan struct{}), cancel: cancel,
		state: State{Status: "Watching launch score"}}
	go s.run(ctx)
	go func() {
		defer close(s.watchDone)
		watcher.Run(ctx, opts.Source, s.SaveNotification, opts.IncludeDirs, func(event watcher.Event) {
			s.mu.Lock()
			s.state.WatchWarning, s.state.WatchedFiles = event.Warning, event.Files
			s.notify()
			s.mu.Unlock()
			if event.Changed {
				s.Request(false)
			}
		})
	}()
	return s, nil
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
	s.state.Generation++
	s.state.Status, s.state.Latest = "Queued", nil
	s.wanted, s.immediate = true, immediate
	s.notify()
	s.mu.Unlock()
	select {
	case s.requests <- struct{}{}:
	default:
	}
}

func (s *Service) Cancel() {
	s.mu.Lock()
	s.state.Generation++
	s.state.Status = "Canceled"
	s.wanted = false
	s.notify()
	s.mu.Unlock()
	select {
	case s.requests <- struct{}{}:
	default:
	}
}

func (s *Service) Close() {
	s.closeOnce.Do(func() {
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
				delay := s.Debounce
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
					complete <- compile(buildCtx, s.Options, s.Root, generation)
				}()
			}
		}
	}
}
