package app

import (
	"io"
	"sync"

	"github.com/muesli/cancelreader"
)

// rawReader joins an active read before releasing the cancel reader's pipes.
// Bubble Tea commands may finish after Program.Run returns, or even start late;
// the lock and closed flag keep those reads from racing with Close.
type rawReader struct {
	mu     sync.Mutex
	reader cancelreader.CancelReader
	closed bool
}

func newRawReader(in io.Reader) (*rawReader, error) {
	r, err := cancelreader.NewReader(in)
	if err != nil {
		return nil, err
	}
	return &rawReader{reader: r}, nil
}

func (r *rawReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, cancelreader.ErrCanceled
	}
	return r.reader.Read(p)
}

func (r *rawReader) Close() error {
	r.reader.Cancel() // Unblock Read before waiting for its lock.
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.reader.Close()
}
