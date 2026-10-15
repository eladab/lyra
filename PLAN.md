# Implementation plan

Initial target: macOS Apple Silicon and Ghostty. One Go workstation embeds
traditional Vim, engraving and playback. Phase boundaries are acceptance gates,
not permission to weaken the integrated terminal architecture.

## Phase 1 — Vim PTY and terminal emulation

Implemented packages: `cmd/lyra`, `internal/app`, `internal/editor`, `internal/input`.

- [x] Bubble Tea v2/Lip Gloss shell and responsive editor/status panes.
- [x] Validate traditional Vim; launch it as a foreground PTY child with normal rc.
- [x] VT screen-state engine, styled cells, alternate screen, cursor and replies.
- [x] Raw byte forwarding, ordered asynchronous writes, prefix and paste routing.
- [x] Editor/preview/help focus; keep ordinary Vim keys and mappings available.
- [x] Pane resize propagated to emulator and kernel PTY, then Vim through SIGWINCH.
- [x] Normal quit, unsaved confirmation, forced cleanup, child reaping and errors.
- [x] Unit, real-Vim and outer-TUI tests, race checks and build verification.

Acceptance: edit/save a path containing spaces; use normal/insert/visual/command
modes; prove `.vimrc` loading; preserve Unicode, tabs and controls; prevent preview
input from entering Vim; prove actual Vim dimensions after resize; restore the
host terminal on ordinary exit and external SIGTERM. Real graphical-terminal
visual qualification remains part of Phase 5. See `docs/VERIFICATION.md`.

## Phase 2 — Async LilyPond builds and diagnostics

Implemented packages: `internal/lilypond`, `internal/watcher`; integrated with
the existing Vim child, input router and Bubble Tea model.

- [x] Serialized compiler worker with context, generation IDs, a configurable
  timeout and process-group cancellation; explicit executable/argument arrays.
- [x] Initial build, additive session-local Vim save notifications, manual rebuild
  (`Ctrl+\ b`) and cancel (`Ctrl+\ c`); no changes to user Vim configuration.
- [x] Fixed launch-score policy, parent-directory watches, 200 ms debounce,
  one-second fingerprint fallback, atomic rename and missing-parent recovery.
- [x] Recursive literal project includes and repeatable `--include` directories.
  Computed/runtime-modified includes and installed libraries require manual
  rebuilds; scan limits and unsupported computed includes produce pane notices.
- [x] Per-generation temporary output, bounded combined logs, source diagnostics
  with multiline continuations and filenames containing spaces/colons.
- [x] Publish only the current successful generation; retain prior good PDF/MIDI
  after errors, cancellation and stale completions; clean temporary files on exit.
- [x] Scrollable build status/diagnostics/raw log, useful missing-tool/source,
  timeout and no-artifact states; editing remains independent of engraving.
- [x] Fake compiler, real Vim save hook, full-app save/manual rebuild, race tests
  and real LilyPond 2.26.0 PDF/MIDI/nested-include/error verification on macOS arm64.

Acceptance: repeated rapid saves produce one current result; saving during a build
does not block input; errors point to source locations; a failed/stale generation
cannot replace a good preview. Test atomic renames, include changes, filenames,
missing LilyPond, cancellation, cleanup and event ordering using fake compiler
processes plus at least one real LilyPond score on macOS.

## Phase 3 — Integrated PDF score preview

**Implementation is present; the native visual acceptance gate is pending.**

- [x] Real selected-page Poppler rasterization, cancellation, owned PDF copy,
  metadata, bounded page cache and viewport cropping off the UI thread.
- [x] Replaceable backend with owned image/placement IDs and bounded Kitty PNG
  chunks; support probe and cell/window pixel replies routed away from Vim.
- [x] Public Ultraviolet cell diffs and graphics composed into one `tea.Raw`
  frame; no styled-text APC injection, concurrent output writer or library fork.
- [x] Page, zoom, scroll, reload, last-good retention and diagnostics/help switching.
- [x] Default half-width split, prefix resize/reset, proportion retained on window
  resize, opt-in mouse divider drag and translated editor/preview mouse events.
- [x] Real LilyPond multi-page PDF plus Poppler/PNG tests and full Bubble Tea outer
  PTY tests for placement bounds, pages, zoom, redraw, resizing and both exit paths.
- [ ] Actual Ghostty and Kitty visual matrix below. The computer-use tool rejected
  access to Ghostty in this session; byte-stream tests are not visual acceptance.

The protocol spike uses the actual Bubble Tea output path. Documentation and
automated frame tests do not establish validated native interoperability.

1. Rasterize one selected PDF page with Poppler (`pdftoppm`) into PNG off the UI
   thread. Cache by build generation, page and effective pixel scale, with bounded
   disk/memory use and cancellation. Render only needed pages/tiles.
2. Define a preview backend independent of the model: probe, display/place,
   delete/clear, resize and close. Keep PDF rasterization separate from graphics.
3. Probe Kitty graphics with bounded responses; use geometry queries for pixel
   dimensions where available. Route responses through a protocol-aware host
   reader before raw Vim forwarding. Environment variables are hints only.
4. Transmit PNG data with owned image/placement IDs and controlled cursor position.
   Render/clip the bitmap to preview bounds before upload; avoid any assumption
   that a character rectangle automatically clips terminal images.
5. Coordinate image placement with Bubble Tea's frame diff and clear operations.
   Use one serialized output owner/compositor; never write graphics concurrently
   with renderer stdout. Evaluate a renderer extension or explicit post-frame
   hook. Do not concatenate APC payloads into styled text and assume preservation.
6. Verify display/delete/reload, zoom, scrolling, page navigation, narrow layout,
   editor focus, pane resize, full redraw and alternate-screen exit in Ghostty and
   Kitty. Then investigate tmux passthrough, SSH and alternate terminal protocols.
7. Preserve page/zoom/position after successful rebuild when valid. Clearly report
   unsupported graphics; editor, diagnostics and playback must still function.

Acceptance: actual multi-page LilyPond engraving remains wholly inside the preview
pane, never leaves stale images after page/zoom/resize/exit, and reloads only good
generations. Test a real Ghostty and Kitty session, not only emitted byte strings.
Text status is an acceptable temporary unsupported-terminal state; an external
PDF window is not the completed product.

## Phase 4 — MIDI synthesis and transport

- [x] Asynchronous generation-owned MIDI copies and FluidSynth rendering to
  validated stereo 44.1 kHz signed 16-bit WAV, with timeout/cancellation and limits.
- [x] Configurable executables, SoundFont and audio output; pinned GeneralUser GS
  2.0.3 installer, SHA-256 validation and preserved license after license review.
- [x] Private mpv JSON IPC with request IDs, bounded I/O, no player window or
  terminal reader, play/pause, stop, seek, volume, timeline, EOF and crash recovery.
- [x] No automatic playback or interruption on save; preserve the playing/paused
  take, offer explicit latest-take playback, reject stale synthesis and clean up.
- [x] Prefix/preview/audio controls, visible status and useful no-MIDI/tool/bank
  diagnostics without interfering with ordinary Vim input.
- [x] Real LilyPond/FluidSynth/mpv tests, complete outer-TUI transport tests,
  race checks, stalled-output detection/retry and process/socket cleanup.
- [ ] Consistent native macOS output qualification: CoreAudio initially passed,
  then stalled at zero in both Lyra's test and standalone mpv. AVFoundation's
  clock advanced but pause hung. See the verification notes; do not count silent
  output tests as native speaker acceptance.

1. Detect MIDI from the successful compiler generation; absence is a normal state
   when a score has no `\midi` block. Keep artifact generations paired.
2. Validate a configured user-owned SoundFont path; provide a useful missing-file
   error and document licensing. Do not assume a downloaded SoundFont is reusable.
3. Run FluidSynth offline synthesis to a temporary WAV asynchronously. Verify CLI
   arguments against the installed version; set sample format/rate explicitly.
4. Control mpv through a private Unix socket using JSON IPC for play/pause, stop,
   seek, volume, duration and time position. Hide video/external windows and keep
   all transport state in Lyra. Use macOS-compatible output, initially CoreAudio.
5. Retain playing audio across compilation; make an explicit generation switch on
   user action or a safe boundary. Do not autoplay or interrupt a performance on
   every save. Handle stale synthesis, EOF, player failure and socket teardown.
6. Later compare direct FluidSynth real-time MIDI playback if latency/tempo needs
   justify the extra transport/control complexity.

Acceptance: play/pause/stop and seeking work with visible position/duration, no
separate player UI, no audio by default, sensible behavior with no MIDI or missing
SoundFont, and no compiler/synth/player processes left on exit. Test unit state
transitions plus a real score, SoundFont and macOS audio output.

## Phase 5 — Configuration, compatibility and release polish

Implemented appearance groundwork: a session-local LilyPond palette with
`--vim-style user` opt-out; warm/night/original PDF themes with `--preview-theme`
and prefix/direct PDF `t`; styled header/pane bars with unchanged row geometry.
Configuration persistence and native terminal qualification remain below.
No-argument sessions also create a collision-safe named temporary sketch, prompt
through Vim on first save, adopt the permanent path for compilation/watchers,
and retain interrupted drafts/recovery files.

- Add versioned TOML config for executables, theme, compile-on-save, debounce,
  renderer preference, SoundFont, audio backend and configurable prefix/bindings.
  Use platform config directories, predictable precedence and useful errors.
  Existing CLI overrides include `--soundfont`, `--preview-theme` and
  `--vim-style`; retain predictable precedence when saved config is added.
  Consider reopening the last score with consent.
- Add diagnostics navigation to Vim without shell interpolation. Consider inverse
  source links, include navigation and stable view positions after rebuild.
- Implement mouse only with an emulator mode bridge, pane hit testing and translated
  coordinates. Preserve shell escape, plugin subprocess and job-control behavior
  under an explicit tested policy.
- Qualify Ghostty/Kitty, dark/light themes, ASCII/combining/wide text, keyboard
  layouts, large pastes, plugins, mouse, remote sessions and tmux passthrough.
- Add reproducible CI, dependency/license review, setup diagnostics, release
  packaging and arm64 smoke tests. Expand Linux support only after runtime tests.
- Measure input latency and redraw CPU on long scores; optimize changed regions
  and snapshot frequency without bypassing screen-state correctness.

Release acceptance: a musician can install dependencies, open an existing score,
edit with their Vim setup, save and see engraving/diagnostics, navigate pages/zoom,
audition MIDI and exit cleanly using only one terminal interface.
