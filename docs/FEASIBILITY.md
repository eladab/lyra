# Feasibility assessment

Investigated 2026-10-15. Local platform: macOS Apple Silicon; Go 1.26.5;
Homebrew traditional Vim 9.2, patches 1–850. Distinguish implemented editor
behavior from the future graphics and audio design below.

## Embedded traditional Vim: feasible, implemented

Vim is a terminal program, so it can run under a controlling PTY without a remote
API. `creack/pty` creates the session/controlling terminal, sets the initial size,
and exposes the master. It documents the nonblocking setup needed for interruptible
I/O. Lyra duplicates and re-wraps the nonblocking descriptor so Go's poller owns
it correctly on macOS. Real-process tests caught the failure mode where changing
only the original descriptor flags yielded EAGAIN and an empty screen.

Source: [creack/pty API and examples](https://pkg.go.dev/github.com/creack/pty).

A PTY is only transport. Lyra uses Charm's `x/vt` emulator to parse terminal output
into screen cells. It renders an atomic styled snapshot through Bubble Tea and
positions the host cursor relative to the editor pane. Cursor moves, erases,
scrolling and alternate-buffer commands remain inside the virtual terminal.
Terminal responses return to Vim through a dedicated response drain and ordered
input writer, preventing query deadlocks. The wrapper owns synchronization rather
than assuming every method on an embedded concurrency wrapper is safe together.

`x/vt` v0.1.0 provides the emulator API; it is an experimental dependency and is
pinned. Its terminal defaults and extension coverage are not a promise of complete
xterm/Ghostty compatibility. Tests cover the sequences essential to this milestone.

Sources: [x/vt package](https://pkg.go.dev/github.com/charmbracelet/x/vt),
[Charm experimental package policy](https://github.com/charmbracelet/x).

Bubble Tea v2.0.10 is pinned. Lyra disables its input reader so key decoding and
host capability queries do not compete with raw Vim input. This version explicitly
skips capability queries with input disabled. Lyra owns raw mode and a cancelable
reader while Bubble Tea owns host drawing and SIGWINCH handling.

Source: [Bubble Tea releases](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.10).

Vim starts normally with `-f -T xterm-256color -- <absolute file>`, inheriting HOME,
VIMINIT and normal runtime configuration. `-f` keeps it in the foreground; no clean
startup or replacement rc is injected. The executable's version banner must
identify traditional Vim. The child sees Lyra's virtual terminal identity instead
of host graphics/multiplexer identity. This preserves configuration loading while
preventing unsupported host capabilities from being advertised as available.

Source: [Vim startup documentation](https://github.com/vim/vim/blob/master/runtime/doc/starting.txt).

## Integrated PDF graphics: plausible, still needs an in-app spike

Ghostty documents Kitty graphics protocol support. Kitty's protocol can transmit
raster images, query support, control placements, and delete images. PDF is not
the terminal image format: rasterize a selected PDF page into PNG first. The
protocol is a plausible backend for both Ghostty and Kitty.

Sources: [Ghostty features](https://ghostty.org/docs/features),
[Kitty graphics specification](https://sw.kovidgoyal.net/kitty/graphics-protocol/).

**Inference:** a reliable preview is achievable, but terminal support alone does
not establish compatibility with Bubble Tea's renderer. The image lives in a
different terminal layer than text cells. A redraw, erase or cursor movement can
invalidate its placement, or an image can cover the editor if bounds are wrong.
Phase 3 now implements a compositor using the public Ultraviolet renderer and
Bubble Tea's `Raw` output path, with one frame in flight at a time. Graphics probe,
geometry routing, crop, delete/reload and terminal cleanup have automated PTY/PNG
coverage. Actual Ghostty/Kitty visual testing remains unproven; the available
computer-use tool denied Ghostty access. See `VERIFICATION.md` for the pending gate.

The Phase 3 spike must resolve:

| Risk | Required mitigation/evidence |
| --- | --- |
| Capability replies become Vim keystrokes | Route host replies before forwarding; bounded probe and fallback |
| Styled-text parsing drops APC graphics sequences | Explicit renderer/compositor output hook; prove byte preservation |
| Image exceeds pane bounds | Raster crop/scale to pane pixels and verify editor stays uncovered |
| Stale placements after redraw/page/zoom | Owned IDs; explicit delete/recreate lifecycle and resize tests |
| Cell dimensions differ by font/DPI | Query geometry when possible and recompute effective pixel scale |
| tmux/SSH changes protocol transport | Qualify later; stream PNG for remote use; do not assume shared paths |
| Unsupported terminal | Useful status/diagnostics and ongoing editing; replaceable backend |

Start with Poppler `pdftoppm`, selected-page PNG output and bounded caching.
Ghostty/Kitty are the first backends. Investigate Sixel as an optional later backend.
Unicode approximations may indicate status, but cannot meet engraved-notation
acceptance. A separate PDF application is outside the required architecture.

## Compilation and audio: conventional subprocess boundaries

Phase 2 implements a background compiler worker with generation-specific temporary
output folders, cancellation of stale requests, serialized subprocess ownership,
and retention of the last successful PDF/MIDI generation. The source directory is
the process working directory and user include directories are explicit arguments.
Real LilyPond 2.26.0 on macOS arm64 verified generation and nested relative includes.
The filename is absolute: real testing found that LilyPond treats a `--` separator
as an additional source filename, unlike Vim.

Containing-directory watches survive atomic saves; a one-second fingerprint poll
recovers missed events and directory replacements. An additive session-local
`BufWritePost` notification catches launch-score saves without editing `.vimrc`.
The target stays the launch score even when Vim changes buffers. Recursive literal
project includes are tracked; computed includes, runtime search-path changes and
installed LilyPond library files require manual rebuilding. See the README for
scan and log limits, temporary-output lifetime and controls.

Sources: [LilyPond command-line usage](https://lilypond.org/doc/v2.24/Documentation/usage/command_002dline-usage),
[fsnotify guidance](https://github.com/fsnotify/fsnotify).

Phase 4 implements offline FluidSynth MIDI-to-WAV rendering followed by mpv JSON
IPC transport. Real GeneralUser GS synthesis, seeking, pause, volume, EOF, player
recovery and complete TUI controls pass with silent mpv output. A five-second
progress watchdog reports device stalls and reaps the player for explicit retry.
Native speaker output qualification remains open: CoreAudio initially passed,
then stalled in both the integration check and standalone mpv; AVFoundation
advanced but hung on pause. See the recorded versions and evidence in
`VERIFICATION.md`.

Source: [FluidSynth file renderer](https://www.fluidsynth.org/api/FileRenderer.html).

## Remaining Phase 1 limits and qualification work

The byte router reserves Ctrl+\ and treats bracketed-paste contents as opaque.
It delays an incomplete escape delimiter for 25 ms. Arbitrarily slow split escape
sequences and terminals already using an enhanced keyboard mode need additional
qualification. The first version intentionally leaves mouse reporting off.

Default color responses are virtual defaults; synchronizing them with host theme
queries is future work. Clipboard/image escape extensions from Vim plugins are
not passed directly to the host. Wide/combining glyph behavior depends on agreement
between emulator and host width rules. Loaded Vim configuration cannot guarantee
support for every plugin's terminal extension.

Vim runs in its own session. Normal quits preserve modified-buffer confirmation;
disconnect and signal cleanup use its process group. Plugins or shell commands
that detach into other groups, suspension/job control, and terminal-mode plugins
need further testing. No userspace application can restore the terminal after
SIGKILL. The implementation bounds its ordered input backlog at 8 MiB and reports
overflow instead of silently dropping input.

These are compatibility boundaries to qualify, not reasons to substitute Neovim,
omit terminal emulation or move the score into a different application.
