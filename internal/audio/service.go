//go:build darwin || linux

package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

type Action uint8

const (
	Toggle Action = iota
	Stop
	Backward
	Forward
	Restart
	Latest
	Louder
	Quieter
)

type State struct {
	Status                                        string
	Generation, SourceGeneration, ReadyGeneration uint64
	HasMIDI, Preparing, QueuedPlay                bool
	Position, Duration                            float64
	Volume                                        int
	Error                                         string
}

// Service's actor owns playback decisions and children. Requests only enqueue;
// no terminal UI callback waits for synthesis, socket I/O or an audio device.
type Service struct {
	opts             Options
	root             string
	mu               sync.Mutex
	state            State
	source           Source
	sources, changed chan struct{}
	actions          chan Action
	done             chan struct{}
	cancel           context.CancelFunc
	closeOnce        sync.Once
}

func NewService(parent context.Context, opts Options) (*Service, error) {
	opts, err := resolveOptions(opts)
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "lyra-audio-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{opts: opts, root: root, state: State{Status: "Waiting for MIDI", Volume: 70}, sources: make(chan struct{}, 1), changed: make(chan struct{}, 1), actions: make(chan Action, 32), done: make(chan struct{}), cancel: cancel}
	go s.run(ctx)
	return s, nil
}
func (s *Service) UpdateSource(source Source) {
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
	select {
	case s.sources <- struct{}{}:
	default:
	}
}
func (s *Service) Request(action Action) error {
	select {
	case <-s.done:
		return errors.New("playback is closed")
	default:
	}
	select {
	case s.actions <- action:
		return nil
	default:
		return errors.New("playback command queue is full")
	}
}
func (s *Service) Snapshot() State          { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Service) Changed() <-chan struct{} { return s.changed }
func (s *Service) Done() <-chan struct{}    { return s.done }
func (s *Service) Close()                   { s.closeOnce.Do(func() { s.cancel(); <-s.done }) }

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	defer os.RemoveAll(s.root)
	state := s.state
	publish := func() {
		s.mu.Lock()
		s.state = state
		s.mu.Unlock()
		select {
		case s.changed <- struct{}{}:
		default:
		}
	}
	var source Source
	var pending *Source
	var ready, loaded *take
	var p *player
	var playerDone <-chan struct{}
	var lastProgress time.Time
	var renderCancel context.CancelFunc
	running := false
	complete := make(chan renderResult, 1)
	defer func() {
		if renderCancel != nil {
			renderCancel()
		}
		if running {
			<-complete
		}
		if p != nil {
			p.close()
		}
	}()
	removeUnused := func(t *take) {
		if t != nil && t != ready && t != loaded {
			_ = os.RemoveAll(t.dir)
		}
	}
	failPlayer := func(err error) {
		state.Error = err.Error()
		state.Status = "Playback error"
		if p != nil {
			p.close()
			p = nil
			playerDone = nil
		}
		old := loaded
		loaded = nil
		removeUnused(old)
	}
	play := func(t *take) error {
		state.Status = "Loading audio"
		state.Error = ""
		publish()
		if p == nil {
			var err error
			p, err = startPlayer(ctx, s.opts)
			if err != nil {
				return err
			}
			playerDone = p.done
		}
		if loaded != t {
			if err := p.load(ctx, t.wav, state.Volume); err != nil {
				return err
			}
			old := loaded
			loaded = t
			removeUnused(old)
		} else {
			if _, err := p.call(ctx, "seek", 0, "absolute+exact"); err != nil {
				return err
			}
		}
		if _, err := p.call(ctx, "set_property", "pause", false); err != nil {
			return err
		}
		state.Generation, state.Position, state.Duration = t.generation, 0, t.duration
		state.Status = "Playing"
		lastProgress = time.Now()
		state.QueuedPlay = false
		return nil
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.sources:
			s.mu.Lock()
			next := s.source
			s.mu.Unlock()
			if next.Generation == source.Generation {
				continue
			}
			source = next
			state.SourceGeneration = source.Generation
			state.HasMIDI = source.Open != nil
			state.Error = ""
			state.QueuedPlay = false
			if renderCancel != nil {
				renderCancel()
			}
			pending = nil
			if source.Open != nil {
				copy := source
				pending = &copy
				state.Preparing = true
			} else {
				state.Preparing = false
				old := ready
				ready = nil
				state.ReadyGeneration = 0
				removeUnused(old)
			}
			if state.Status != "Playing" && state.Status != "Paused" {
				state.Position = 0
				state.Generation = 0
				state.Duration = 0
				if state.HasMIDI {
					state.Status = "Preparing audio"
				} else {
					state.Status = "No MIDI: add a MIDI block"
				}
			}
			publish()
		case result := <-complete:
			renderCancel()
			renderCancel = nil
			running = false
			s.mu.Lock()
			wanted := s.source.Generation
			s.mu.Unlock()
			if result.generation != source.Generation || result.generation != wanted || !state.HasMIDI {
				removeUnused(result.take)
				break
			}
			state.Preparing = false
			if result.err != nil {
				state.Error = "Synthesis: " + result.err.Error()
				state.QueuedPlay = false
				if state.Status != "Playing" && state.Status != "Paused" {
					state.Status = "Synthesis error"
				}
			} else {
				old := ready
				ready = result.take
				state.ReadyGeneration = ready.generation
				removeUnused(old)
				if state.Status != "Playing" && state.Status != "Paused" {
					state.Status = "Ready"
					state.Generation = ready.generation
					state.Position = 0
					state.Duration = ready.duration
				}
				if state.QueuedPlay {
					if err := play(ready); err != nil {
						failPlayer(err)
					}
				}
			}
			publish()
		case <-playerDone:
			failPlayer(fmt.Errorf("mpv exited unexpectedly: %v: %s", p.err, p.log.String()))
			publish()
		case action := <-s.actions:
			state.Error = ""
			var err error
			switch action {
			case Toggle, Latest:
				if action == Toggle && p != nil && loaded != nil && (state.Status == "Playing" || state.Status == "Paused") {
					state.QueuedPlay = false
					pause := state.Status == "Playing"
					_, err = p.call(ctx, "set_property", "pause", pause)
					if err == nil {
						if pause {
							state.Status = "Paused"
						} else {
							state.Status = "Playing"
							lastProgress = time.Now()
						}
					}
				} else if ready != nil && ready.generation == source.Generation && state.HasMIDI {
					err = play(ready)
				} else if state.Preparing {
					if action == Latest {
						state.QueuedPlay = true
					} else {
						state.QueuedPlay = !state.QueuedPlay
					}
				} else if !state.HasMIDI {
					state.Error = "No MIDI: add \\midi { } to the score and save"
				} else {
					state.Error = "Audio is not ready; rebuild the score to retry synthesis"
				}
			case Stop:
				state.QueuedPlay = false
				if p != nil && loaded != nil {
					_, err = p.call(ctx, "set_property", "pause", true)
					if err == nil {
						_, err = p.call(ctx, "seek", 0, "absolute+exact")
					}
					state.Generation, state.Duration = loaded.generation, loaded.duration
				}
				state.Position = 0
				state.Status = "Stopped"
			case Backward, Forward, Restart:
				if p != nil && loaded != nil && state.Generation == loaded.generation {
					target := state.Position
					if action == Backward {
						target -= 5
					} else if action == Forward {
						target += 5
					} else {
						target = 0
					}
					target = math.Max(0, math.Min(target, loaded.duration))
					_, err = p.call(ctx, "seek", target, "absolute+exact")
					if err == nil {
						state.Position = target
						lastProgress = time.Now()
						if state.Status == "Ended" {
							state.Status = "Paused"
						}
					}
				}
			case Louder, Quieter:
				volume := state.Volume + 5
				if action == Quieter {
					volume = state.Volume - 5
				}
				volume = max(0, min(100, volume))
				if p != nil {
					_, err = p.call(ctx, "set_property", "volume", volume)
				}
				if err == nil {
					state.Volume = volume
				}
			}
			if err != nil {
				failPlayer(err)
			}
			publish()
		case <-ticker.C:
			if p == nil || loaded == nil || (state.Status != "Playing" && state.Status != "Paused") {
				continue
			}
			data, err := p.call(ctx, "get_property", "time-pos")
			if err != nil {
				failPlayer(err)
				publish()
				continue
			}
			var position float64
			if json.Unmarshal(data, &position) == nil && !math.IsNaN(position) && !math.IsInf(position, 0) {
				if math.Abs(position-state.Position) > .01 {
					lastProgress = time.Now()
				}
				state.Position = math.Max(0, math.Min(position, loaded.duration))
			}
			data, err = p.call(ctx, "get_property", "eof-reached")
			if err != nil {
				failPlayer(err)
				publish()
				continue
			}
			var eof bool
			if json.Unmarshal(data, &eof) == nil && eof {
				state.Status = "Ended"
				state.Position = loaded.duration
			} else if state.Status == "Playing" && time.Since(lastProgress) > 5*time.Second {
				failPlayer(errors.New("audio output is not advancing; check the output device or select another driver with --audio-output, then Play to retry"))
			}
			publish()
		}
		if !running && pending != nil {
			next := *pending
			pending = nil
			job, cancel := context.WithCancel(ctx)
			renderCancel = cancel
			running = true
			go func() { defer cancel(); complete <- render(job, s.opts, s.root, next) }()
		}
	}
}
