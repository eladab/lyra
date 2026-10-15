# Lyra project instructions

## Product and non-negotiable requirements

Lyra is a Go terminal-native LilyPond music workstation. It embeds traditional
Vim as a genuine child process, rather than implementing Vim keybindings or
depending on a Vim plugin. Initial target: macOS Apple Silicon, with Ghostty as
the first graphics terminal to qualify. The CLI is `lyra [composition.ly]`.

- Use Bubble Tea and Lip Gloss for one keyboard-first integrated TUI.
- Run traditional Vim in a PTY with a real VT terminal-emulation layer between
  child output and the host renderer. A PTY by itself is insufficient.
- Do not replace Vim with Neovim. Verify custom executables as well as PATH Vim.
- Preserve normal `.vimrc`, plugins, mappings, syntax, colors, startup environment,
  swap and recovery behavior. Do not use `--clean`, `-u NONE`, or rewrite user
  configuration. Test-specific clean HOME directories are appropriate in tests.
- Preserve ordinary Vim input. Tab, Ctrl+C, Ctrl+Q, Ctrl+S, function keys, Unicode,
  and escape sequences must not become default Lyra global shortcuts. Keep
  application controls behind an explicit prefix or outside editor focus.
- Engrave asynchronously with LilyPond on save, show diagnostics, keep the last
  good result, and support manual rebuilds. The UI must remain responsive.
- Display actual engraved PDF pages inside the terminal, with reload, page
  navigation, zoom and scrolling. A separate PDF application is not a substitute.
- Support LilyPond MIDI through FluidSynth and a configurable SoundFont. Provide
  play/pause, stop, seek, current position and duration. An intermediate WAV and
  mpv control are acceptable for the first audio implementation.
- Inspect existing files before changing them. Do not discard unrelated edits,
  replace user scores, create assumed remotes, or alter the user's Vim setup.

## Current implementation and scope

Phase 1 implements the application shell, Vim PTY, VT parsing and screen rendering,
cursor, raw input forwarding, prefix routing, focus, responsive layout, resizing,
exit reporting, process reaping and terminal restoration. Phase 2 adds asynchronous
LilyPond compilation, a session-local Vim save hook, parent-directory/include
watching with polling fallback, diagnostics and retained good PDF/MIDI outputs.
The build target remains the launch score when Vim changes buffers. Phase 3 adds
asynchronous Poppler PDF rasterization, a replaceable Kitty graphics backend,
serialized text/image frames, page/zoom/scroll/reload controls and an adjustable
50/50 split. Mouse divider dragging and scrolling are opt-in. Real PDF and outer
PTY tests pass; actual Ghostty/Kitty visual qualification remains pending. Phase 4
adds an asynchronous FluidSynth MIDI-to-WAV worker and mpv JSON IPC transport,
with play/pause/stop/seek/volume, timeline, explicit take switching and stalled
output recovery. Real synthesis/transport tests pass; native speaker output
qualification remains open because standalone mpv also stalls on this machine.
GeneralUser GS is the optional, checksum-pinned installed default.
Appearance polish adds a session-local LilyPond syntax/palette overlay (opt out
with `--vim-style user`), warm/night/original-paper PDF themes, and styled chrome
without adding terminal rows. Never edit the user's Vim configuration for this.
Running without a filename shows a startup chooser to browse existing `.ly`
scores or choose a folder and filename for a new score. Suggest a musical name,
but create nothing until the user confirms the destination. New files must be
created exclusively without replacing existing paths. The chosen score is a
permanent file and the fixed editor/header/build target; Vim saves it normally
from the first `:w`. Named launches bypass the chooser. The chooser must join
its sole raw reader and restore terminal modes before the editor reader starts.

The module name is `lyra`, a local module path, pending a chosen repository URL.
Pinned dependencies are in `go.mod` and `go.sum`. Go 1.26.5 is the initial build
baseline. macOS/arm64 is the tested runtime; Linux has build-compatible PTY code
but is not a qualified release target yet. Windows is outside this milestone.

## Architecture and ownership

```text
host terminal input -> cancelable raw reader -> prefix/paste router
                                                  |
                              Bubble Tea model ---+-- focus and app actions
                                                  |
                                           ordered input queue
                                                  |
                                    PTY <-> traditional Vim
                                     |
                                output reader
                                     |
                           locked VT emulator -> atomic snapshot
                                     |                  |
                            terminal query replies      v
                                     +-> input queue  Bubble Tea + Lip Gloss
```

- `cmd/lyra`: CLI parsing and external signal context.
- `internal/app`: terminal setup/restoration, startup file chooser, Bubble Tea
  model, pane geometry, view, focus and event dispatch.
- `internal/editor`: executable validation, PTY/process ownership, input queue,
  screen emulator and query response bridge.
- `internal/input`: incremental byte router; bracketed-paste payloads are opaque.
- `internal/lilypond`: serialized builds, generation ownership, logs and diagnostics.
- `internal/watcher`: directory events, polling and literal include discovery.
- `internal/preview`: bounded Poppler worker, document ownership and Kitty backend.
- `internal/audio`: synthesis, bounded take ownership, private mpv control/lifecycle.
- `internal/draft`: musical name suggestions and private temporary score utilities.
- `internal/testdata`: stable scores for integration tests; never depend on mutable
  user-edited examples or change those scores to make tests pass.
- `examples/scale.ly`: a small score that includes both layout and MIDI blocks.
- `docs/FEASIBILITY.md`: investigation, sources and remaining protocol risks.
- `PLAN.md`: five phases, acceptance criteria and the next concrete steps.

Keep later responsibilities in `internal/lilypond`, `internal/watcher`,
`internal/preview`, `internal/audio` and `internal/config` when implemented.
Do not create fake implementations that report compilation or playback success.

The model owns UI state. PTY reader/writer goroutines own I/O. Exactly one goroutine
calls `Cmd.Wait`. The terminal wrapper serializes writes/resizes/snapshots. Query
replies must be drained independently of that lock. Child screen escape sequences
must never be forwarded directly to host stdout. Bubble Tea owns host output.

The host reader bypasses Bubble Tea's key decoder with `WithInput(nil)`; Lyra owns
raw mode and cancelable reads. Do not add a second stdin reader or enable host
capability queries without explicitly routing their replies away from Vim.
The frame compositor uses the public Ultraviolet text renderer and `tea.Raw` with
`WithoutRenderer`; one acknowledged frame is in flight at a time. Lyra routes
SIGWINCH explicitly because WithoutRenderer disables Bubble Tea's TTY discovery.
Never insert APC graphics into styled text. Clear only Lyra's owned image ID,
restore images after an ED2 text redraw, and clean up modes after Tea output stops.

## Keys and layout

`Ctrl+\` is a two-keystroke command prefix: `e` editor, `p` preview, `h`/`?` help,
`q` request quit through Vim, `b` rebuild the saved launch score, `c` cancel
engraving, and another `Ctrl+\` sends a literal prefix byte.
Phase 3 adds prefix `d` diagnostics, `[`/`]` narrower/wider editor, `=` half split,
and `m` toggle mouse reporting. The split proportion survives window resizing.
Prefix `t`, or direct `t` in PDF focus, cycles warm/night/original paper. The
initial appearance can be selected with `--preview-theme warm|night|paper`.
Phase 4 adds prefix Space play/pause, s stop, comma/period seek backward/forward
five seconds, u play latest take and a audio pane. In PDF/audio focus these
transport keys are direct. Audio focus adds +/- volume and 0 restart; PDF focus
keeps its zoom and fit keys. Plain editor input remains untouched.
Unknown prefix combinations are passed through. In preview, `q` or Esc returns
to Vim. Ordinary keys in preview do not edit the source. `:q`, `:wq`, `:qa` and Vim
confirmation retain Vim's normal semantics. Do not intercept Tab or `q` in Vim.
In the build pane, j/k scroll, g/G go to the first/last section and l toggles the
raw compiler log. No new unprefixed editor shortcuts are reserved.
In PDF focus, n/p navigate pages, +/- zoom, 0 resets the current fit, f switches
content-width/full-page fit, hjkl/arrows scroll, r reloads/redraws, and d opens
diagnostics. Default content fit trims blank margins and starts at the top;
preserve footer/other printed content for scrolling and never rewrite the PDF.
Mouse events are translated before
the emulator encodes them using Vim's own mouse mode; never forward host reports
as raw editor input. Bracketed-paste payloads remain opaque to host reply parsing.

The header and pane headings occupy the first two rows; the footer occupies the
last row. Vim's emulator and PTY use exactly the editor content dimensions.
At 88 columns and above, show the side pane. Below that, show the focused pane.
Do not resize the editor just because focus changes. Handle tiny windows without
negative dimensions. Translate mouse coordinates before any future mouse support.

Phase 5 must make the prefix and application bindings configurable. Future compile,
transport, page and zoom controls belong under the prefix or in preview focus;
do not silently reserve F5–F8 or Ctrl+Q from existing user mappings.

## Process, data and configuration rules

- Invoke commands with explicit executable and argument arrays, never shell
  interpolation of score paths, SoundFonts, includes or diagnostics.
- Pass source paths after `--` to Vim. Retain working-directory semantics.
- Child `TERM=xterm-256color`, `TERM_PROGRAM=lyra`, `COLORTERM=truecolor` describe
  the emulated terminal. Remove inherited LINES/COLUMNS and terminal/multiplexer
  identifiers that could cause incorrect capability assumptions. Keep HOME,
  VIMINIT/EXINIT, runtime paths and ordinary plugin environment intact.
- A user quit must ask Vim to quit and honor modified-buffer confirmation.
  Signal/disconnect cleanup may send SIGHUP, then bounded SIGKILL if needed.
  Preserve Vim's normal swap recovery; forced process termination cannot promise
  saving unsaved edits. Do not claim SIGKILL can restore terminal settings.
- Keep input queue memory bounded and report errors. Do not block the UI on PTY
  writes, engraving, PDF rasterization or synthesis.
- Build artifacts belong in `bin/` or per-run temporary directories. Later
  compiler outputs must not overwrite the user's existing PDFs/MIDI by default.
- Watch parent directories to survive atomic-save renames. Debounce, serialize
  builds and reject stale results by generation. Include-file tracking belongs in
  Phase 2, including explicit policy for changing the current Vim buffer.
- Never bundle a SoundFont without checking its license. Missing optional tools
  must produce useful pane diagnostics while editing continues.
- Never autoplay on save or replace a playing/paused take when synthesis finishes.
  Latest explicitly loads/plays the newest ready take. Stop clears queued play.
  New generations clear queued play too; first MIDI output is the current policy.
- Keep mpv's private socket in a 0700 directory, disable its own terminal I/O and
  scripts/config, match JSON request IDs, check command errors and bound waits.
  Reap synthesizer/player once and remove takes only when no longer referenced.

## Verification and definition of done

For editor/lifecycle changes, run `go vet ./...`, `go test -race ./...` and
`go build -o bin/lyra ./cmd/lyra`. Tests use a temporary HOME and real traditional
Vim. Missing Vim skips PTY integration tests; a release must not count skips as
successful integration verification. Keep external-tool tests deterministic.

Phase 1 acceptance:

1. A real Vim child edits and saves a file in the embedded pane with ordinary
   insert, normal, visual and command-line modes and user's configuration loaded.
2. Styled screen cells, cursor, alternate buffers and terminal query replies go
   through the emulator; no child cursor command can reposition host chrome.
3. Input and bracketed paste survive chunk boundaries; application prefix actions
   and preview focus cannot leak into the score.
4. Resizes reach both the emulator and kernel PTY and are observed by Vim.
5. Normal exit, unsaved confirmation/cancel, unexpected child death and external
   termination reap the child and restore the outer terminal where possible.
6. Race checks pass; tests include an outer PTY running the entire Bubble Tea app.

Do not claim graphics interoperability from documentation alone. Before Phase 3
is accepted, test actual Ghostty and Kitty with image placement, clipping,
clear/delete, full redraw, zoom, page changes, resize, exit and host response
routing. Keep the renderer replaceable; text status is only a temporary state.
