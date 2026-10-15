package input

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Only recognized terminal replies are consumed. Everything inside bracketed
// paste is opaque. Partial controls are bounded, and Flush preserves a lone Esc.
func hostEvent(p []byte) (Event, int, bool) {
	for _, start := range []string{"\x1b_G", "\x1b[<", "\x1b[6;", "\x1b[4;"} {
		if len(p) < len(start) && bytes.HasPrefix([]byte(start), p) {
			return Event{}, 0, true
		}
	}
	if bytes.HasPrefix(p, []byte("\x1b_G")) {
		end := bytes.Index(p, []byte("\x1b\\"))
		if end < 0 {
			if len(p) >= 4096 {
				return Event{Action: HostReply, Code: -1}, len(p), false
			}
			return Event{}, 0, true
		}
		if end >= 4096 {
			return Event{Action: HostReply}, end + 2, false
		}
		return Event{Action: HostReply, Data: append([]byte(nil), p[:end+2]...)}, end + 2, false
	}
	if bytes.HasPrefix(p, []byte("\x1b[<")) {
		for i := 3; i < len(p); i++ {
			if p[i] == 'M' || p[i] == 'm' {
				var b, x, y int
				if n, _ := fmt.Sscanf(string(p[3:i]), "%d;%d;%d", &b, &x, &y); n == 3 && x > 0 && y > 0 {
					return Event{Action: MouseEvent, Code: b, X: x - 1, Y: y - 1, Release: p[i] == 'm'}, i + 1, false
				}
				return Event{}, 0, false
			}
			if (p[i] < '0' || p[i] > '9') && p[i] != ';' {
				return Event{}, 0, false
			}
		}
		return Event{}, 0, len(p) < 64
	}
	if bytes.HasPrefix(p, []byte("\x1b[6;")) || bytes.HasPrefix(p, []byte("\x1b[4;")) {
		for i := 4; i < len(p); i++ {
			if p[i] == 't' {
				parts := strings.Split(string(p[2:i]), ";")
				if len(parts) == 3 {
					h, e1 := strconv.Atoi(parts[1])
					w, e2 := strconv.Atoi(parts[2])
					if e1 == nil && e2 == nil {
						return Event{Action: HostReply, Code: int(p[2] - '0'), X: w, Y: h}, i + 1, false
					}
				}
				return Event{}, 0, false
			}
			if (p[i] < '0' || p[i] > '9') && p[i] != ';' {
				return Event{}, 0, false
			}
		}
		return Event{}, 0, len(p) < 64
	}
	return Event{}, 0, false
}
