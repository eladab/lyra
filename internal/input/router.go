// Package input routes raw terminal bytes without decoding and re-encoding keys.
package input

import "bytes"

const Prefix byte = 0x1c // Ctrl+\

type Action uint8

const (
	None Action = iota
	Editor
	Preview
	Help
	Quit
	Build
	CancelBuild
	Narrower
	Wider
	EqualSplit
	Diagnostics
	ToggleMouse
	HostReply
	MouseEvent
	Audio
	PlayPause
	StopAudio
	SeekBack
	SeekForward
	LatestAudio
	PreviewTheme
	Documentation
)

type Event struct {
	Data   []byte
	Action Action
	// Host control replies and SGR mouse reports are kept out of Vim input.
	Code, X, Y int
	Release    bool
}

type Router struct {
	pending                   []byte
	paste                     bool
	prefix                    bool
	discardAPC, discardEscape bool
}

// Feed retains partial paste delimiters and the command prefix across reads.
// Prefix bytes in a bracketed paste are always literal editor input.
func (r *Router) Feed(data []byte) []Event {
	r.pending = append(r.pending, data...)
	var events []Event
	appendByte := func(b byte) {
		if len(events) == 0 || events[len(events)-1].Action != None {
			events = append(events, Event{})
		}
		i := len(events) - 1
		events[i].Data = append(events[i].Data, b)
	}
	for len(r.pending) > 0 {
		if r.discardAPC {
			b := r.pending[0]
			r.pending = r.pending[1:]
			if r.discardEscape && b == '\\' {
				r.discardAPC = false
			}
			r.discardEscape = b == 0x1b
			continue
		}
		marker := []byte("\x1b[200~")
		if r.paste {
			marker = []byte("\x1b[201~")
		}
		if bytes.HasPrefix(marker, r.pending) && len(r.pending) < len(marker) {
			break
		}
		if bytes.HasPrefix(r.pending, marker) {
			for _, b := range marker {
				appendByte(b)
			}
			r.pending = r.pending[len(marker):]
			r.paste = !r.paste
			continue
		}
		b := r.pending[0]
		if !r.paste && b == 0x1b {
			e, n, wait := hostEvent(r.pending)
			if wait {
				break
			}
			if n > 0 {
				if e.Code == -1 {
					r.discardAPC = true
					r.discardEscape = r.pending[n-1] == 0x1b
				}
				events = append(events, e)
				r.pending = r.pending[n:]
				continue
			}
		}
		r.pending = r.pending[1:]
		if r.paste {
			appendByte(b)
			continue
		}
		if r.prefix {
			r.prefix = false
			action := map[byte]Action{'e': Editor, 'p': Preview, 'h': Help, '?': Help, 'q': Quit, 'b': Build, 'c': CancelBuild, '[': Narrower, ']': Wider, '=': EqualSplit, 'd': Diagnostics, 'm': ToggleMouse, 'a': Audio, ' ': PlayPause, 's': StopAudio, ',': SeekBack, '.': SeekForward, 'u': LatestAudio, 't': PreviewTheme, 'D': Documentation}[b]
			if action != None {
				events = append(events, Event{Action: action})
				continue
			}
			appendByte(Prefix)
			if b != Prefix {
				appendByte(b)
			}
			continue
		}
		if b == Prefix {
			r.prefix = true
			continue
		}
		appendByte(b)
	}
	return events
}

// Flush forwards an incomplete escape sequence after a short idle period. A
// lone Esc must reach Vim even when it is also the start of a paste delimiter.
func (r *Router) Flush() []Event {
	if len(r.pending) == 0 {
		return nil
	}
	// Recognizable replies may arrive slowly (SSH, overloaded terminals).
	// Never turn a partial terminal reply into an editor command on idle.
	if !r.paste && (bytes.HasPrefix(r.pending, []byte("\x1b_G")) || bytes.HasPrefix(r.pending, []byte("\x1b[<")) || bytes.HasPrefix(r.pending, []byte("\x1b[6;")) || bytes.HasPrefix(r.pending, []byte("\x1b[4;"))) {
		return nil
	}
	var data []byte
	if r.prefix {
		data = append(data, Prefix)
		r.prefix = false
	}
	e := Event{Data: append(data, r.pending...)}
	r.pending = nil
	return []Event{e}
}
