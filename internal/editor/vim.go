//go:build darwin || linux

package editor

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

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

type Options struct {
	Vim, File string
	// DisableLilyPondSyntax preserves the user's LilyPond colors unchanged.
	// By default a session-local LilyPond palette is added after normal startup.
	DisableLilyPondSyntax bool
	// SaveNotification enables an additive session-local BufWritePost hook.
	SaveNotification string
	Cols, Rows       int
	// Env is primarily useful to run isolated Vim integration tests. Nil
	// inherits the user's environment, including HOME and VIMINIT.
	Env []string
}

type Session struct {
	Terminal    *Terminal
	master      *os.File
	cmd         *exec.Cmd
	writeMu     sync.Mutex
	inputQueue  [][]byte
	queuedBytes int
	inputReady  chan struct{}
	writerDone  chan struct{}
	closeOnce   sync.Once
	changed     chan struct{}
	done        chan struct{}
	processDone chan struct{}
	readDone    chan struct{}
	faults      chan error
	err         error  // published by closing done
	syntaxDir   string // session-local, removed after the Vim child exits
}

// ResolveVim rejects Neovim even when an executable or symlink is named vim.
func ResolveVim(command string) (string, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("find traditional Vim %q: %w", command, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("check Vim version: %w", err)
	}
	if !strings.HasPrefix(string(out), "VIM - Vi IMproved") {
		return "", fmt.Errorf("%s is not traditional Vim; Neovim is not supported", path)
	}
	return path, nil
}

func Start(opts Options) (*Session, error) {
	if opts.Cols < 1 || opts.Rows < 1 || opts.Cols > 65535 || opts.Rows > 65535 {
		return nil, fmt.Errorf("invalid editor dimensions: %dx%d", opts.Cols, opts.Rows)
	}
	command := opts.Vim
	if command == "" {
		command = "vim"
	}
	path, err := ResolveVim(command)
	if err != nil {
		return nil, err
	}
	file, err := filepath.Abs(opts.File)
	if err != nil {
		return nil, err
	}
	// No -u NONE, -U NONE, --clean, -n, injected mappings, or shell command.
	args := []string{"-f", "-T", "xterm-256color"}
	var syntaxDir string
	if !opts.DisableLilyPondSyntax {
		var script string
		syntaxDir, script, err = writeSyntaxOverlay()
		if err != nil {
			return nil, err
		}
		args = append(args, "-S", script)
	}
	started := false
	defer func() {
		if !started && syntaxDir != "" {
			_ = os.RemoveAll(syntaxDir)
		}
	}()
	if opts.SaveNotification != "" {
		script := filepath.Join(filepath.Dir(opts.SaveNotification), "save.vim")
		contents := fmt.Sprintf("augroup LyraSession%d\n  autocmd BufWritePost * if resolve(expand('<afile>:p')) ==# resolve($LYRA_SOURCE_FILE) | silent! call writefile([reltimestr(reltime())], $LYRA_SAVE_FILE) | endif\naugroup END\n", os.Getpid())
		if err := os.WriteFile(script, []byte(contents), 0600); err != nil {
			return nil, fmt.Errorf("create save notification hook: %w", err)
		}
		args = append(args, "-S", script)
	}
	args = append(args, "--", file)
	cmd := exec.Command(path, args...)
	cmd.Env = childEnvironment(opts.Env)
	if opts.SaveNotification != "" {
		cmd.Env = append(cmd.Env, "LYRA_SOURCE_FILE="+file, "LYRA_SAVE_FILE="+opts.SaveNotification)
	}
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(opts.Cols), Rows: uint16(opts.Rows)})
	if err != nil {
		return nil, fmt.Errorf("start Vim PTY: %w", err)
	}
	// Re-wrap a nonblocking duplicate so os.NewFile registers it with Go's
	// poller. Changing flags on the original blocking File alone can produce
	// EAGAIN reads instead of poller waits on macOS.
	fd, err := syscall.Dup(int(master.Fd()))
	_ = master.Close()
	if err == nil {
		err = syscall.SetNonblock(fd, true)
	}
	if err != nil {
		if fd >= 0 {
			_ = syscall.Close(fd)
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("configure PTY: %w", err)
	}
	master = os.NewFile(uintptr(fd), "lyra-vim-pty")
	s := &Session{master: master, cmd: cmd, changed: make(chan struct{}, 1), done: make(chan struct{}),
		inputReady: make(chan struct{}, 1), writerDone: make(chan struct{}),
		processDone: make(chan struct{}), readDone: make(chan struct{}), faults: make(chan error, 2), syntaxDir: syntaxDir}
	s.Terminal = newTerminal(opts.Cols, opts.Rows, s.WriteInput)
	go s.writeInput()
	go s.readOutput()
	go s.wait()
	started = true
	return s, nil
}

func childEnvironment(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env)+3)
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "TERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION", "COLORTERM", "LINES", "COLUMNS", "KITTY_WINDOW_ID", "TMUX", "STY":
			continue
		}
		out = append(out, item)
	}
	return append(out, "TERM=xterm-256color", "TERM_PROGRAM=lyra", "COLORTERM=truecolor")
}

func (s *Session) readOutput() {
	defer close(s.readDone)
	buf := make([]byte, 32*1024)
	for {
		n, err := s.master.Read(buf)
		if n > 0 {
			_, _ = s.Terminal.Write(buf[:n])
			select {
			case s.changed <- struct{}{}:
			default:
			}
		}
		if err != nil {
			s.reportIOError("read Vim output", err)
			return
		}
	}
}

func (s *Session) reportIOError(operation string, err error) {
	// EOF/EIO are expected when the slave closes, potentially just before Wait.
	if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
		return
	}
	select {
	case <-s.processDone:
		return
	default:
	}
	select {
	case s.faults <- fmt.Errorf("%s: %w", operation, err):
	default:
	}
	_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
}

func (s *Session) wait() {
	s.err = s.cmd.Wait() // exactly one owner reaps the child
	close(s.processDone)
	// Drain the final screen, but bound waiting for plugins that retain the PTY.
	select {
	case <-s.readDone:
	case <-time.After(250 * time.Millisecond):
	}
	_ = s.master.Close()
	<-s.readDone
	<-s.writerDone
	for len(s.faults) > 0 {
		s.err = errors.Join(s.err, <-s.faults)
	}
	s.Terminal.Close()
	if s.syntaxDir != "" {
		_ = os.RemoveAll(s.syntaxDir)
	}
	close(s.done)
}

func (s *Session) Changed() <-chan struct{} { return s.changed }
func (s *Session) Done() <-chan struct{}    { return s.done }
func (s *Session) Err() error               { <-s.done; return s.err }

func (s *Session) WriteInput(p []byte) error {
	select {
	case <-s.processDone:
		return os.ErrClosed
	default:
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	// Bound queued paste data while keeping a slow Vim from blocking Update.
	if s.queuedBytes+len(p) > 8*1024*1024 {
		return errors.New("Vim input backlog exceeded 8 MiB")
	}
	s.inputQueue = append(s.inputQueue, append([]byte(nil), p...))
	s.queuedBytes += len(p)
	select {
	case s.inputReady <- struct{}{}:
	default:
	}
	return nil
}

func (s *Session) writeInput() {
	defer close(s.writerDone)
	for {
		select {
		case <-s.processDone:
			return
		case <-s.inputReady:
		}
		for {
			s.writeMu.Lock()
			if len(s.inputQueue) == 0 {
				s.writeMu.Unlock()
				break
			}
			p := s.inputQueue[0]
			s.inputQueue[0] = nil
			s.inputQueue = s.inputQueue[1:]
			s.queuedBytes -= len(p)
			s.writeMu.Unlock()
			if err := s.writeAll(p); err != nil {
				s.reportIOError("write Vim input", err)
				return
			}
		}
	}
}

func (s *Session) writeAll(p []byte) error {
	for len(p) > 0 {
		n, err := s.master.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 65535 || rows > 65535 {
		return errors.New("invalid PTY size")
	}
	select {
	case <-s.processDone:
		return os.ErrClosed
	default:
	}
	conn, err := s.master.SyscallConn()
	if err != nil {
		return err
	}
	// Update the emulator first: Vim can redraw as soon as SIGWINCH arrives.
	s.Terminal.Resize(cols, rows)
	// Pin the descriptor for the ioctl. pty.Setsize calls File.Fd directly,
	// which races Close and can change a nonblocking PTY back to blocking.
	var resizeErr error
	err = conn.Control(func(fd uintptr) {
		resizeErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)})
	})
	return errors.Join(err, resizeErr)
}

// RequestQuit lets Vim handle modified buffers and confirmation itself.
func (s *Session) RequestQuit() error {
	return s.WriteInput([]byte("\x1c\x0e:confirm qall\r"))
}

// Close handles terminal loss, signals, and failed startup of the outer TUI.
// A normal interactive quit instead goes through Vim's :q/:wq/:confirm qall.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		select {
		case <-s.processDone:
			<-s.done
			return
		default:
		}
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGCONT)
		select {
		case <-s.done:
			return
		case <-time.After(500 * time.Millisecond):
		}
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		_ = s.cmd.Process.Kill()
		_ = s.master.Close()
		<-s.done
	})
}
