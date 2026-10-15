//go:build darwin || linux

package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type message struct {
	RequestID uint64          `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	Event     string          `json:"event"`
	Reason    string          `json:"reason"`
}
type player struct {
	cmd            *exec.Cmd
	conn           net.Conn
	dir            string
	cancel         context.CancelFunc
	done, readDone chan struct{}
	err            error // published by done
	log            *boundedLog
	mu, writeMu    sync.Mutex
	next           uint64
	pending        map[uint64]chan message
	events         chan message
}

func startPlayer(ctx context.Context, opts Options) (*player, error) {
	// Keep the private socket path short enough for macOS sockaddr_un, even
	// when TMPDIR or a test directory has a long name. MkdirTemp uses 0700.
	dir, err := os.MkdirTemp("/tmp", "lyra-player-")
	if err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	p := &player{dir: dir, cancel: cancel, done: make(chan struct{}), readDone: make(chan struct{}), log: &boundedLog{}, pending: make(map[uint64]chan message), events: make(chan message, 32)}
	socket := dir + "/ipc"
	args := []string{"--no-config", "--load-scripts=no", "--idle=yes", "--pause=yes", "--terminal=no", "--input-terminal=no", "--input-default-bindings=no", "--input-vo-keyboard=no", "--input-media-keys=no", "--media-controls=no", "--force-window=no", "--video=no", "--audio-display=no", "--keep-open=yes", "--volume=70", "--input-ipc-server=" + socket}
	if opts.Output != "" {
		args = append(args, "--ao="+opts.Output)
	}
	p.cmd = exec.CommandContext(child, opts.MPV, args...)
	configureProcess(p.cmd)
	p.cmd.WaitDelay = time.Second
	p.cmd.Stdout, p.cmd.Stderr = p.log, p.log
	if err := p.cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}
	go func() { p.err = p.cmd.Wait(); close(p.done) }()
	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialTimeout("unix", socket, 50*time.Millisecond)
		if err == nil {
			p.conn = conn
			go p.read()
			return p, nil
		}
		select {
		case <-ctx.Done():
			cancel()
			<-p.done
			_ = os.RemoveAll(dir)
			return nil, ctx.Err()
		case <-p.done:
			cancel()
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("mpv exited: %v: %s", p.err, p.log.String())
		case <-deadline.C:
			cancel()
			<-p.done
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("mpv IPC startup timed out: %s", p.log.String())
		case <-ticker.C:
		}
	}
}

func (p *player) read() {
	defer close(p.readDone)
	scanner := bufio.NewScanner(p.conn)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		var msg message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			break
		}
		if msg.RequestID != 0 {
			p.mu.Lock()
			reply := p.pending[msg.RequestID]
			p.mu.Unlock()
			if reply != nil {
				select {
				case reply <- msg:
				default:
				}
			}
			continue
		}
		if msg.Event == "file-loaded" || msg.Event == "end-file" {
			select {
			case p.events <- msg:
			default:
				p.cancel()
				return
			}
		}
	}
}

func (p *player) call(ctx context.Context, args ...any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	p.mu.Lock()
	p.next++
	id := p.next
	reply := make(chan message, 1)
	p.pending[id] = reply
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.pending, id); p.mu.Unlock() }()
	data, err := json.Marshal(struct {
		Command   []any  `json:"command"`
		RequestID uint64 `json:"request_id"`
	}{args, id})
	if err != nil {
		return nil, err
	}
	p.writeMu.Lock()
	_ = p.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = p.conn.Write(append(data, '\n'))
	p.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("mpv IPC write: %w", err)
	}
	select {
	case msg := <-reply:
		if msg.Error != "success" {
			return nil, fmt.Errorf("mpv %v: %s", args[0], msg.Error)
		}
		return msg.Data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("mpv IPC: %w", ctx.Err())
	case <-p.readDone:
		return nil, errors.New("mpv IPC disconnected")
	}
}

func (p *player) load(ctx context.Context, wav string, volume int) error {
	for len(p.events) > 0 {
		<-p.events
	}
	if _, err := p.call(ctx, "set_property", "pause", true); err != nil {
		return err
	}
	if _, err := p.call(ctx, "set_property", "volume", volume); err != nil {
		return err
	}
	if _, err := p.call(ctx, "loadfile", wav, "replace"); err != nil {
		return err
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-p.events:
			if msg.Event == "file-loaded" {
				return nil
			}
			if msg.Event == "end-file" && msg.Reason == "error" {
				return fmt.Errorf("mpv could not load synthesized audio: %s", p.log.String())
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-p.readDone:
			return fmt.Errorf("mpv disconnected loading audio: %s", p.log.String())
		case <-timer.C:
			return errors.New("mpv audio loading timed out")
		}
	}
}

func (p *player) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	_, _ = p.call(ctx, "quit")
	cancel()
	p.cancel()
	_ = p.conn.Close()
	<-p.done
	<-p.readDone
	_ = os.RemoveAll(p.dir)
}
