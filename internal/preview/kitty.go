// Package preview rasterizes real PDFs and provides a replaceable terminal
// graphics backend. It never writes to the terminal itself.
package preview

import (
	"fmt"
	"strings"
)

// Backend operations are composed with text into a single Bubble Tea Raw frame.
type Backend interface {
	Probe() string
	Delete() string
	Draw(payload string, x, y, cols, rows int) string
}

// Kitty owns exactly one image ID per session, never terminal-global deletes.
type Kitty struct{ ID uint32 }

func (k Kitty) Probe() string {
	return fmt.Sprintf("\x1b_Ga=q,i=%d,s=1,v=1,f=24;AAAA\x1b\\\x1b[16t\x1b[14t", k.ID)
}
func (k Kitty) Delete() string { return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", k.ID) }
func (k Kitty) Draw(payload string, x, y, cols, rows int) string {
	if payload == "" || cols < 1 || rows < 1 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b7\x1b[%d;%dH", y+1, x+1)
	for pos := 0; pos < len(payload); {
		end := min(pos+4096, len(payload))
		more := 0
		if end < len(payload) {
			more = 1
		}
		if pos == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,t=d,i=%d,p=1,c=%d,r=%d,C=1,q=2,m=%d;", k.ID, cols, rows, more)
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d,q=2;", more)
		}
		b.WriteString(payload[pos:end])
		b.WriteString("\x1b\\")
		pos = end
	}
	b.WriteString("\x1b8")
	return b.String()
}
