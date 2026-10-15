# Phase 1–4 and appearance verification

Date: 2026-10-15. Platform: macOS/arm64. Toolchain: Go 1.26.5.
Editor: `/opt/homebrew/bin/vim`, traditional Vim 9.2, patches 1–850.
Engraver: Homebrew LilyPond 2.26.0 at `/opt/homebrew/bin/lilypond`. The official
macOS arm64 archive was also used for initial Phase 2 verification.
Rasterizer: Homebrew Poppler 26.10.0 (`pdfinfo` and `pdftoppm`).
Audio: Homebrew FluidSynth 2.6.1, mpv 0.41.0, GeneralUser GS 2.0.3.

## Automated results

| Check | Result |
| --- | --- |
| `go vet ./...` | Passed |
| `LYRA_TEST_LILYPOND=... LYRA_TEST_SOUNDFONT=... go test -race -timeout 90s ./...` | All nine packages, including real Vim, LilyPond, Poppler, FluidSynth and mpv integration |
| `LYRA_TEST_MACOS_AUDIO=1 ... go test -race -run TestMacOSAudioOutput ./internal/audio` | Qualification open: two early CoreAudio checks passed, later runs stalled; see playback evidence |
| `go build -o bin/lyra ./cmd/lyra` | Passed, native macOS arm64 binary |
| `./bin/lyra --help` | Passed |
| `make deps` | Passed; Homebrew Go/Vim/LilyPond reused and Poppler installed |
| `make install` | Passed with all six Brewfile dependencies and verified default SoundFont; installed binary matches `bin/lyra` at `~/.local/bin/lyra` |
| `GOOS=linux GOARCH=amd64 go build ...` | Passed; cross-build compatibility only |
| Real compiler test with `/opt/homebrew/bin/lilypond` | Passed under the race detector; PDF/MIDI, nested includes and invalid-source diagnostics |

Coverage includes:

- A real Vim process running under a controlling PTY, with an isolated test
  `.vimrc` loaded through normal startup rather than a replacement `-u` option.
- Insert, normal, visual and command-line editing, Unicode, tabs, save/quit,
  paths with spaces and a leading dash, arrow keys and a user-defined F5 mapping.
- Bracketed paste in real Vim; router tests cover every two-part delimiter split
  and one-byte delivery, with application-prefix bytes kept literal in paste.
- VT cursor positioning, RGB styling, alternate-screen restoration and DSR replies.
- Concurrent emulation, resize and snapshot access under the race detector.
- Resizing the kernel PTY and reading Vim's actual `&columns` and `&lines` back.
- Requesting quit with an unsaved buffer, observing Vim's confirmation, canceling,
  then saving. Normal exit, unexpected child death and forced cleanup are tested.
- Launching the entire Bubble Tea application inside an outer PTY, editing and
  saving through it, ignoring text while preview has focus, returning to Vim,
  shrinking the terminal, and exiting normally or with external SIGTERM.
- Decoding actual host output into a second VT screen and checking headings,
  empty-buffer tilde alignment, the full pane divider, preview text and footer.
  This reproduces and prevents the diagonal drift caused by enabling raw mode
  while Bubble Tea's disabled-input renderer assumes kernel LF-to-CRLF mapping.
- Raw control/Unicode input and mapped newline output, with shared or separate
  input/output terminals; original output processing is restored on exit.
- Joining pending input reads before closing the cancel reader's resources.
- Restoration of terminal input/output flags, control characters and speeds, and exit
  from the alternate screen. The test excludes the kernel-managed PENDIN flag,
  which Darwin may set while processing a terminal-settings update.
- Missing Vim, a fake Neovim executable named `vim`, invalid CLI arguments and
  redirected input/output are rejected with errors.

Test scores, HOME directories, config markers and output files are temporary.
The user's `.vimrc` and existing scores were not changed by verification.

Phase 2 checks additionally cover:

- Real LilyPond PDF/MIDI generation from `internal/testdata/scale.ly` in temporary output
  directories, nested relative includes, source filenames with spaces, colons and
  leading dashes, and parsed errors from a deliberately invalid score.
- Real Vim's additive save hook, unchanged-file saves, preservation of normal
  config, and ignoring saves of a different active buffer.
- Automatic compilation after saving through the entire TUI and manual rebuilds
  through the raw input prefix; editing and resize checks remain in the same test.
- CLI-owned signal cancellation avoids a duplicate Bubble Tea signal handler
  blocking shutdown after the context has canceled the event loop.
- Fake compiler failures, missing executable/source, missing artifacts, MIDI-only
  and PDF-only output, 256 KiB log limits and timeouts with partial output.
- Rapid requests during a slow build, stale-result rejection, retention of the
  last good output after failure/cancel, bounded cleanup and temporary removal.
- Literal include discovery, comment/string exclusion, cycles, computed-include
  notices, atomic renames, and missing/replaced include-parent directories.
- Build-pane escaping, wrapping and scrolling. No compiler control sequence can
  become host terminal control output.

The real compiler uses temporary HOME and XDG cache directories. Its production
invocation omits `--`: LilyPond 2.26 treats that token as a source filename. The
source path is absolute, so a leading dash in its basename cannot become an option.
Vim still receives its supported `--` separator.

## Investigation versus runtime evidence

The embedded editor has been exercised with real nested PTYs on this Mac.
An interactive human visual check in Ghostty/Kitty has not been performed.
Phase 1 tests validate screen state and emitted terminal frames, not pixel-perfect
rendering in every host terminal, font, theme or keyboard layout.

LilyPond and FluidSynth were not found on PATH during initial setup. The official
[LilyPond download](https://lilypond.org/download.html) was first used from a
temporary directory for Phase 2 tests. The user-requested `make install` now
installs the Brewfile dependencies, including LilyPond, and places Lyra in
`~/.local/bin`. Both commands were verified on PATH and the Homebrew compiler
passed the real integration test. FluidSynth and mpv are now playback dependencies.
PDF rasterization and the graphics frame path are now implemented and tested as
described below. Ghostty's documented Kitty graphics
support is research evidence; native visual interoperability remains unverified.

## Phase 3 automated evidence

- Poppler reads and renders a real two-page PDF, with exact viewport PNG
  dimensions, distinct page contents and a measured shape that detects stretching.
- Zoom/cropping/scroll limits, page-count clamping, missing Poppler and bounded
  viewport errors are covered. A superseded sleeping rasterizer is canceled and
  cannot publish over a newer request.
- Content fit enlarges notation to the pane width and trims paper margins using
  a cached 1000-pixel page measurement followed by a sharper raster. A PDF with
  widely separated music and footer verifies that the footer cannot shrink the
  music, remains reachable by scrolling, and preserves the music's aspect ratio.
  Blank pages and crop invalidation after a new generation are covered. The f
  toggle and zoom/reset controls preserve a full-page option.
  The current `tutorial/durations.ly` was engraved into a private build-output
  directory, and both fit modes' viewport PNGs were visually checked: content
  fit fills the width with notation, with small margins and no stretching.
- After the original PDF is removed, subsequent pages still render from the
  preview-owned document. The compiler exposes a descriptor opened under its
  publication lock, remaining valid across generation-directory replacement.
- Real LilyPond `internal/testdata/scale.ly` PDF is rendered to PNG with visible notation
  ink. The outer TUI test additionally engraves two distinct bookpart pages using
  real LilyPond, then uses a deterministic compiler fixture for save/rebuild events.
- `examples/preview-pages.ly` engraves successfully to a two-page PDF. Both
  Poppler page PNGs were visually inspected for correct titles and music. These
  raster checks do not qualify their placement inside a graphical terminal.
- The complete Bubble Tea app in an outer PTY receives a simulated Kitty support
  acknowledgement and pixel geometry. Actual PNG chunks emitted through `tea.Raw`
  are decoded and checked against preview cell bounds. Pages, zoom, scrolling,
  reload, diagnostics clearing, split resize, normal exit and SIGTERM are tested.
  This harness is not a real Ghostty/Kitty graphics renderer.
- Frame tests cover owned IDs, 4096-byte chunk limits, cursor restoration, image
  removal on diagnostics/narrow editor/tiny layout, full ED2 redraw with image
  restoration, proportion changes and mode cleanup.
- Input tests split graphics replies, geometry and SGR mouse reports at every
  boundary; preserve bracketed paste; handle slow replies and replies interrupting
  a pending prefix; bound oversized graphics responses without forwarding them.
- Keyboard split reset/adjustment, opt-in divider dragging/release, preview wheel
  routing and unchanged editor geometry on focus changes are covered. Rapid split
  changes exposed a File.Fd/Close race: PTY resizing now uses SyscallConn.Control
  to pin the descriptor for the ioctl, including concurrent child exit.

## Phase 4 playback evidence

- Real LilyPond creates MIDI from the stable integration score. Real FluidSynth
  renders that MIDI with GeneralUser GS to WAV; its RIFF/PCM fields, sample rate,
  channel count, duration and non-silent sample data are checked.
- Real mpv, using silent output for routine tests, starts only after an explicit
  play request. Its actual position advances, holds on pause, and responds to
  forward/backward seek, restart, stop, volume and EOF. A wrapper records the
  player PID, proving synthesis alone did not start mpv and shutdown reaped it.
- A newer MIDI generation prepares while an earlier take stays paused. Latest
  explicitly switches playback to the new take. Removing MIDI leaves the current
  take playing; subsequent Play after Stop reports the missing MIDI usefully.
- A killed mpv child produces a visible playback error; another Play recreates
  the player using retained ready audio. Failed synthesis, missing WAV output,
  broken SoundFont contents, timeout and stale-cancellation cleanup are covered.
- A second IPC client freezes the real silent-output player while the service
  believes it is playing. After five seconds without position movement, Lyra
  reports stalled output, reaps the player and allows another Play to retry.
- The complete application in an outer PTY compiles/copies real MIDI, synthesizes
  it and starts/pauses/stops real mpv through prefix controls. Save/rebuild, PDF
  navigation, input isolation, resizing, normal exit and SIGTERM share the test.
- Playback router tests preserve ordinary Space/s/comma/period/u and bracketed
  paste at every read boundary. Audio controls use the prefix or pane focus.
- The opt-in native audio test synthesizes a real score at mpv volume 15%, checks
  advancing time, pause, seek/resume and cleanup. Two early CoreAudio checks
  passed, but later runs consistently opened the MacBook Air speakers with zero
  position, despite `pause=false` and `current-ao="coreaudio"`. A standalone
  Python IPC probe reproduced the zero clock with both direct-file playback and
  idle/loadfile playback, independently of Lyra. Matching the device's 48 kHz
  rate and floating-point output did not resolve it. AVFoundation advanced but
  its pause request hung. macOS is 26.6.2; no global audio settings were changed.
  Native output qualification remains open; routine silent-output transport tests
  are successful and do not establish speaker playback.
  macOS Launch Services requires these real-player tests to run outside the
  restricted tool sandbox; the initial sandboxed startup produced an explicit
  Launch Services error and was rerun with execution approval.
- GeneralUser GS license v2.0 was read before installation; the pinned author
  revision is `684543d5e5efaef08d02be50dcda8d552478fa60`. SHA-256 of the bank is
  `9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe`.
  `make soundfont` installed the bank and complete license in the user's Lyra data
  directory. The repository carries license text and installer, not the binary.

Tests now use `internal/testdata/scale.ly`; the current user-edited
`examples/scale.ly` is intentionally preserved even though LilyPond rejects its
second bare expression in the score. `examples/playback.ly` is a separate valid
score for trying the transport. Tests never repair or replace user scores.

## Appearance evidence

- Real traditional Vim tests cover the new palette with and without existing
  LilyPond syntax. Commands, pitches/accidentals, octaves, durations, rests,
  dynamics, strings/comments, markup and nested Scheme receive distinct groups.
  User startup files, plugins, an F5 mapping, shared colors and non-LilyPond
  buffers remain intact; `.ily` is supported and the opt-out is tested.
- Real Poppler preview tests check warm, night and original white output,
  theme-aware cache reuse, unchanged source PDF and crop geometry. Tests also
  cover colored annotations, grayscale/alpha antialiasing and cancellation.
- The outer-PTY application test cycles PDF themes with both prefix and direct
  keys while displaying real PNGs. Prefix tests preserve ordinary Vim `t` and
  bracketed-paste data across read boundaries. Chrome tests cover both focus
  states, wide filenames, tiny windows and exact row widths from 1–180 columns.
- `LYRA_APPEARANCE_ARTIFACTS=/private/tmp/lyra-polish go test -run
  TestAppearanceArtifacts ./internal/app` exports actual styled Vim/model rows
  and real LilyPond/Poppler viewport PNGs. Warm and night composites were visually
  inspected offscreen. This does not establish native graphics interoperability.

## Startup chooser evidence

- CLI accepts zero or one score path and rejects multiple paths. No-argument
  launches show an Open/Create menu; named launches retain the direct editor
  path covered by the existing whole-application tests.
- Browser tests cover sorted folders and `.ly` scores, hidden-file filtering,
  directory symlinks, nonregular-file refusal, Unicode paths, home expansion,
  directory navigation, selected-file opening and asynchronous path completion.
  Stale directory/completion results cannot replace a newer form or edited path.
- New-file tests protect existing scores, reject invalid paths, add a missing
  `.ly` extension, and make twelve concurrent creation attempts at one location:
  exactly one succeeds. Cancelling before confirmation creates nothing.
- View tests cover menu, browser and path forms in tiny and normal terminals,
  long paths, control-bearing filenames, fixed dialog geometry and exact row
  widths. Directory reads run off the UI thread with bounded listings.
- Complete outer-PTY tests use a real traditional Vim and isolated `.vimrc`.
  They open an existing Unicode path, create a new score in a chosen directory,
  refuse replacement of an existing score, resize the chooser before opening,
  and verify Vim receives the selected file and current terminal dimensions.
  Compiler source/cwd and save-triggered rebuilds follow the chosen path, and
  the first `:w` uses ordinary Vim saving. Selected files remain after exit.
- Separate outer-PTY cases quit from the menu and send SIGTERM from the filename
  form. Vim never starts, no new score appears, reader shutdown completes, and
  the helper verifies original input/output terminal settings are restored.

## Pending native visual acceptance

The computer-use tool explicitly rejected access to `com.mitchellh.ghostty` for
safety reasons during this session. No alternative UI automation was used to
bypass that restriction. Actual Ghostty and Kitty visual acceptance has **not**
been completed. Phase 3 must not be described as terminal-qualified until both
hosts pass this matrix and their versions/results are recorded here.

In each host run:

```sh
cd ~/dev/lyra
lyra examples/preview-pages.ly
```

| Action | Expected visual result | Ghostty | Kitty |
| --- | --- | --- | --- |
| Initial build, `Ctrl+\ p` | Engraved content fills right pane width with small margins | Pending | Pending |
| `n`, `p` | Distinct second/first page, no old image remains | Pending | Pending |
| `+`, `j`, `l`, `0`, `f` | Zoom/pan clipped to pane, reset current fit, switch content/full page | Pending | Pending |
| `r` | Full text/image redraw retains divider and cursor | Pending | Pending |
| `Ctrl+\ [` / `]` / `=` | Split adjusts/reset, Vim redraws, PDF never covers editor | Pending | Pending |
| `Ctrl+\ m`, drag divider, wheel/trackpad | Drag adjusts split; preview scrolls; editor mouse obeys Vim modes | Pending | Pending |
| `Ctrl+\ d`, `Ctrl+\ h`, `Ctrl+\ p` | Clear image for text panes, restore when returning | Pending | Pending |
| Resize below 88 columns; `Ctrl+\ e` / `p` | Focused pane fills width; no hidden image over Vim | Pending | Pending |
| Save valid change, then deliberate error, then undo/save | Reload good PDF, retain it on failure, recover | Pending | Pending |
| Normal `:q` / prefix quit, then separate SIGTERM run | No image left behind; terminal modes/cursor restored | Pending | Pending |
| Edit while support/geometry replies arrive | No reply bytes inserted into score | Pending | Pending |

## Manual editor check

```sh
cd ~/dev/lyra
./bin/lyra examples/scale.ly
```

Try your normal mappings and colorscheme, edit then undo, save, switch with
Ctrl+\ then p/e, resize the terminal, use `:split` inside Vim, open `:help`, and
exit with `:q`/`:qa` or the application quit prefix. An unsaved-buffer quit request
should display Vim's own confirmation. Larger plugin setups, mouse and suspension
need additional qualification; see the README's known limits.
