//go:build darwin || linux

package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func expectTerminalBytes(t *testing.T, file *os.File, want []byte) {
	t.Helper()
	reader, err := newRawReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got := make([]byte, len(want))
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(reader, got)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("terminal bytes = %q, want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("terminal input or output was blocked")
	}
}

func TestTerminalModesAndRestoration(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprint("separate-output=", separate), func(t *testing.T) {
			open := func() (*os.File, *os.File) {
				t.Helper()
				master, slave, err := pty.Open()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = master.Close(); _ = slave.Close() })
				return master, slave
			}
			inMaster, in := open()
			outMaster, out := inMaster, in
			if separate {
				outMaster, out = open()
			}
			settings := func(file *os.File) *unix.Termios {
				t.Helper()
				state, err := unix.IoctlGetTermios(int(file.Fd()), termiosGetRequest)
				if err != nil {
					t.Fatal(err)
				}
				state.Lflag &^= unix.PENDIN
				return state
			}
			// Start with different output processing to prove it is both
			// replaced during rendering and restored at the end.
			initial := settings(out)
			initial.Oflag = unix.OPOST | unix.OCRNL
			if err := unix.IoctlSetTermios(int(out.Fd()), termiosSetRequest, initial); err != nil {
				t.Fatal(err)
			}
			beforeIn, beforeOut := settings(in), settings(out)
			restore, err := prepareTerminal(in, out)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = restore() })
			keys := []byte("\t\x03\x11\x13\x1bé\x1c\r\n")
			if _, err := inMaster.Write(keys); err != nil {
				t.Fatal(err)
			}
			expectTerminalBytes(t, in, keys)
			if _, err := out.Write([]byte("A\tB\rC\nD")); err != nil {
				t.Fatal(err)
			}
			expectTerminalBytes(t, outMaster, []byte("A\tB\rC\r\nD"))
			if err := restore(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(beforeIn, settings(in)) || !reflect.DeepEqual(beforeOut, settings(out)) {
				t.Fatal("input or output terminal settings were not restored")
			}
		})
	}
}
