//go:build darwin || linux

package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type draftVim struct {
	*Session
	dir, draft, source, saved string
}

func startDraftVim(t *testing.T, config ...string) draftVim {
	t.Helper()
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("traditional Vim is required for PTY integration tests")
	}
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	draft := filepath.Join(dir, "scratch", "starlit-cadenza.ly")
	if err := os.Mkdir(filepath.Dir(draft), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, nil, 0600); err != nil {
		t.Fatal(err)
	}
	rc := "set nocompatible\nset noswapfile viminfo= nomore\nset shortmess+=I\nset ttimeoutlen=20\n" +
		"let g:pre = 0\nlet g:post = 0\n" +
		"autocmd BufWritePre * let g:pre += 1\n" +
		"autocmd BufWritePost * let g:post += 1\n" +
		"autocmd VimEnter * execute 'cd ' . fnameescape('" + dir + "') | call writefile(['ready'], '" + filepath.Join(dir, "ready") + "')\n"
	rc += strings.Join(config, "\n")
	if err := os.WriteFile(filepath.Join(dir, ".vimrc"), []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "HOME" && key != "VIMINIT" && key != "EXINIT" && key != "GVIMINIT" {
			env = append(env, item)
		}
	}
	env = append(env, "HOME="+dir)
	d := draftVim{dir: dir, draft: draft, source: filepath.Join(dir, "source.json"), saved: filepath.Join(dir, "saved")}
	s, err := Start(Options{File: draft, Cols: 100, Rows: 24, Env: env, Draft: true,
		SourceNotification: d.source, SaveNotification: d.saved, DisableLilyPondSyntax: true})
	if err != nil {
		t.Fatal(err)
	}
	d.Session = s
	t.Cleanup(s.Close)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("screen=%q", ansi.Strip(s.Terminal.Snapshot().Content))
			b, _ := os.ReadFile(d.source)
			t.Logf("source=%s", b)
			states, _ := filepath.Glob(filepath.Join(dir, "state-*"))
			for _, state := range states {
				b, _ = os.ReadFile(state)
				t.Logf("state=%q", b)
			}
		}
	})
	eventually(t, "draft Vim startup", func() bool { return readEquals(filepath.Join(dir, "ready"), "ready\n") })
	return d
}

func (d draftVim) prompt(t *testing.T, command string) {
	t.Helper()
	send(t, d.Session, command+"\r")
	eventually(t, "Vim first-save filename prompt", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "Save score as: starlit-cadenza.ly")
	})
}

func (d draftVim) adopt(t *testing.T, want string) {
	t.Helper()
	eventually(t, "successful first save publishes chosen absolute path", func() bool {
		b, _ := os.ReadFile(d.source)
		var got string
		return json.Unmarshal(b, &got) == nil && got == want
	})
}

func (d draftVim) assertState(t *testing.T, name string, modified bool, pre, post int) {
	t.Helper()
	marker := filepath.Join(d.dir, fmt.Sprintf("state-%d", time.Now().UnixNano()))
	send(t, d.Session, ":call writefile([expand('%:p'), string(&modified), string(g:pre), string(g:post)], '"+marker+"')\r")
	want := fmt.Sprintf("%s\n%d\n%d\n%d\n", name, map[bool]int{false: 0, true: 1}[modified], pre, post)
	eventually(t, "Vim buffer name, modified flag and user write hooks", func() bool { return readEquals(marker, want) })
}

func TestDraftFirstSaveDefaultNameAndNormalLaterWrites(t *testing.T) {
	d := startDraftVim(t)
	send(t, d.Session, "ic4 d e f\x1b")
	d.prompt(t, ":w")
	send(t, d.Session, "\r")
	target := filepath.Join(d.dir, "starlit-cadenza.ly")
	d.adopt(t, target)
	eventually(t, "default name saved against current working directory", func() bool { return readEquals(target, "c4 d e f\n") })
	d.assertState(t, target, false, 1, 1)
	first, _ := os.ReadFile(d.source)
	send(t, d.Session, "Aoops\x1b:w\r")
	eventually(t, "subsequent ordinary write uses adopted filename", func() bool { return readEquals(target, "c4 d e foops\n") })
	d.assertState(t, target, false, 2, 2)
	if b, _ := os.ReadFile(d.source); string(b) != string(first) {
		t.Fatal("source adoption was republished on a later write")
	}
	if !readEquals(d.draft, "") {
		t.Fatal("first save changed temporary draft rather than named file")
	}
}

func TestDraftSwapRecoveryAfterTerminalLoss(t *testing.T) {
	for _, closeSession := range []bool{false, true} {
		t.Run(fmt.Sprintf("close=%v", closeSession), func(t *testing.T) {
			d := startDraftVim(t, "set swapfile directory=. updatecount=1 updatetime=100\n")
			want := "c4 d e f\n% unsaved 雨\n"
			send(t, d.Session, "ic4 d e f\r% unsaved 雨\x1b:preserve\r")
			marker := filepath.Join(d.dir, "swap-state")
			send(t, d.Session, ":call writefile([swapname('%'), string(&swapfile), &buftype], '"+marker+"')\r")
			var swap string
			eventually(t, "real Vim preserves the modified draft into a swap file", func() bool {
				b, _ := os.ReadFile(marker)
				parts := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
				if len(parts) != 3 || parts[0] == "" || parts[1] != "1" || parts[2] != "acwrite" {
					return false
				}
				swap = parts[0]
				b, _ = os.ReadFile(swap)
				return strings.HasPrefix(string(b), "b0VIM ")
			})
			if !readEquals(d.draft, "") {
				t.Fatal("editing or :preserve wrote unsaved text to the draft source")
			}
			if closeSession {
				d.Close()
			} else {
				if err := syscall.Kill(-d.cmd.Process.Pid, syscall.SIGHUP); err != nil {
					t.Fatal(err)
				}
				eventually(t, "SIGHUP reaps the Vim child", func() bool {
					select {
					case <-d.Done():
						return true
					default:
						return false
					}
				})
			}
			if _, err := os.Stat(swap); err != nil {
				t.Fatalf("terminal loss removed the draft swap: %v", err)
			}
			if _, err := os.Stat(d.source); !os.IsNotExist(err) {
				t.Fatal("terminal loss adopted an unsaved draft")
			}

			// Recover with a second genuine Vim process, retaining the isolated
			// HOME. -n prevents creating another swap; -r reads the retained one.
			// Write the recovered lines to a separate file without overwriting
			// either the original draft or the evidence used for recovery.
			recovered := filepath.Join(d.dir, "recovered.ly")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, d.cmd.Path, "-n", "-es", "-r",
				"-c", "call writefile(getline(1, '$'), $LYRA_RECOVERY_OUTPUT)", "-c", "qa!", "--", d.draft)
			cmd.Env = append(append([]string(nil), d.cmd.Env...), "LYRA_RECOVERY_OUTPUT="+recovered)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("recover original draft with traditional Vim: %v\n%s", err, out)
			}
			if !readEquals(recovered, want) {
				b, _ := os.ReadFile(recovered)
				t.Fatalf("recovered text = %q, want %q", b, want)
			}
			if !readEquals(d.draft, "") {
				t.Fatal("recovery verification overwrote the original draft")
			}
		})
	}
}

func TestDraftCancelFailureAndRetryDoNotAdopt(t *testing.T) {
	d := startDraftVim(t)
	send(t, d.Session, "ic4\x1b")
	d.prompt(t, ":wq")
	send(t, d.Session, "\x1b")
	eventually(t, "cancelled first save returns to editor", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "save cancelled")
	})
	select {
	case <-d.Done():
		t.Fatal("cancelled :wq exited Vim")
	default:
	}
	d.assertState(t, d.draft, true, 0, 0)
	if _, err := os.Stat(d.source); !os.IsNotExist(err) {
		t.Fatal("cancelled save adopted a path")
	}

	existing := filepath.Join(d.dir, "existing.ly")
	if err := os.WriteFile(existing, []byte("keep me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	d.prompt(t, ":w!")
	send(t, d.Session, "\x15existing.ly\r")
	eventually(t, "prompt refuses existing file despite ordinary :w!", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "file already exists")
	})
	d.assertState(t, d.draft, true, 0, 0)
	if !readEquals(existing, "keep me\n") {
		t.Fatal("prompt overwrote existing score")
	}

	d.prompt(t, ":w")
	send(t, d.Session, "\x15missing-directory/score.ly\r")
	eventually(t, "failed disk write remains in Vim", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "Can't open file for writing")
	})
	d.assertState(t, d.draft, true, 1, 0)
	if _, err := os.Stat(d.source); !os.IsNotExist(err) {
		t.Fatal("failed save adopted a path")
	}
	d.prompt(t, ":w")
	send(t, d.Session, "\x15nocturne 雨 with spaces.ly\r")
	target := filepath.Join(d.dir, "nocturne 雨 with spaces.ly")
	d.adopt(t, target)
	eventually(t, "Unicode filename with literal spaces", func() bool { return readEquals(target, "c4\n") })
	d.assertState(t, target, false, 2, 1)
}

func TestDraftPromptInterruptAndLiteralFilename(t *testing.T) {
	d := startDraftVim(t)
	send(t, d.Session, "ic4\x1b")
	d.prompt(t, ":wq")
	send(t, d.Session, "\x03")
	eventually(t, "interrupted first save returns to editor", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "save cancelled")
	})
	d.assertState(t, d.draft, true, 0, 0)
	d.prompt(t, ":w")
	name := "rain | 'quoted' % # $() `notes` [1].ly"
	send(t, d.Session, "\x15"+name+"\r")
	d.adopt(t, filepath.Join(d.dir, name))
	eventually(t, "prompt filename characters stay literal", func() bool { return readEquals(filepath.Join(d.dir, name), "c4\n") })
	d.assertState(t, filepath.Join(d.dir, name), false, 1, 1)
}

func TestDraftPromptHomeAndOriginalPath(t *testing.T) {
	for _, original := range []bool{false, true} {
		t.Run(fmt.Sprint(original), func(t *testing.T) {
			d := startDraftVim(t)
			send(t, d.Session, "ic4\x1b")
			d.prompt(t, ":w")
			answer, target := "~/at home.ly", filepath.Join(d.dir, "at home.ly")
			if original {
				answer, target = d.draft, d.draft
			}
			send(t, d.Session, "\x15"+answer+"\r")
			d.adopt(t, target)
			d.assertState(t, target, false, 1, 1)
			eventually(t, "chosen filename saved", func() bool { return readEquals(target, "c4\n") })
		})
	}
}

func TestDraftSourcePublicationFailureCanRetry(t *testing.T) {
	d := startDraftVim(t)
	if err := os.Mkdir(d.source, 0700); err != nil {
		t.Fatal(err)
	}
	send(t, d.Session, "ic4\x1b")
	d.prompt(t, ":wq")
	send(t, d.Session, "\r")
	target := filepath.Join(d.dir, "starlit-cadenza.ly")
	eventually(t, "score written before publication failure", func() bool { return readEquals(target, "c4\n") })
	eventually(t, "publication error leaves Vim open", func() bool {
		return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "source notification could not be published")
	})
	d.assertState(t, target, false, 1, 1)
	if err := os.Remove(d.source); err != nil {
		t.Fatal(err)
	}
	send(t, d.Session, ":w\r")
	d.adopt(t, target)
	d.assertState(t, target, false, 2, 2)
}

func TestDraftCannotSaveOverSessionMarkers(t *testing.T) {
	for _, marker := range []string{"source", "source temporary", "saved"} {
		t.Run(marker, func(t *testing.T) {
			d := startDraftVim(t)
			target := d.source
			if marker == "source temporary" {
				target += ".tmp"
			} else if marker == "saved" {
				target = d.saved
			}
			send(t, d.Session, "ic4\x1b")
			d.prompt(t, ":w")
			send(t, d.Session, "\x15"+target+"\r")
			eventually(t, "internal session path rejected", func() bool {
				return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "outside the internal session files")
			})
			d.assertState(t, d.draft, true, 0, 0)
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("score was written over an internal session path")
			}
		})
	}
}

func TestDraftPreservesCustomWriteCommand(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			d := startDraftVim(t, "let g:writes = 0\nautocmd BufWriteCmd *.ly call writefile(getline(1, '$'), expand('<afile>:p')) | let g:writes += 1 | setlocal nomodified\n")
			send(t, d.Session, "ic4\x1b")
			target := filepath.Join(d.dir, "custom.ly")
			if explicit {
				send(t, d.Session, ":w "+target+"\r")
			} else {
				d.prompt(t, ":w")
				send(t, d.Session, "\x15custom.ly\r")
			}
			d.adopt(t, target)
			send(t, d.Session, "A d8\x1b:w\r")
			eventually(t, "custom writer remains active on later saves", func() bool { return readEquals(target, "c4 d8\n") })
		})
	}
}

func TestDraftExplicitWriteAndSaveAs(t *testing.T) {
	for _, command := range []string{":w", ":saveas", ":w!", ":saveas!"} {
		t.Run(command, func(t *testing.T) {
			d := startDraftVim(t)
			target := filepath.Join(d.dir, "explicit.ly")
			if strings.HasSuffix(command, "!") {
				if err := os.WriteFile(target, []byte("replace me\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			send(t, d.Session, "ic4\x1b"+command+" "+target+"\r")
			d.adopt(t, target)
			d.assertState(t, target, false, 1, 1)
		})
	}
}

func TestDraftUnmodifiedExplicitWriteCannotOverwrite(t *testing.T) {
	for _, command := range []string{":w", ":saveas"} {
		for _, undo := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/undo=%v", command, undo), func(t *testing.T) {
				d := startDraftVim(t)
				target := filepath.Join(d.dir, "existing.ly")
				if err := os.WriteFile(target, []byte("keep me\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if undo {
					send(t, d.Session, "ic4\x1bu")
				}
				d.assertState(t, d.draft, false, 0, 0)
				send(t, d.Session, command+" "+target+"\r")
				marker := filepath.Join(d.dir, "checked")
				send(t, d.Session, ":call writefile(['checked'], '"+marker+"')\r")
				eventually(t, "explicit write completed", func() bool { return readEquals(marker, "checked\n") })
				if !readEquals(target, "keep me\n") {
					t.Fatal("unmodified draft overwrote an existing destination")
				}
				if _, err := os.Stat(d.source); !os.IsNotExist(err) {
					t.Fatal("refused existing destination was adopted")
				}
			})
		}
	}
}

func TestDraftWriteAndQuitAndOtherBuffers(t *testing.T) {
	for _, command := range []string{":wq", ":x", "ZZ"} {
		t.Run(command, func(t *testing.T) {
			d := startDraftVim(t)
			send(t, d.Session, "ic4\x1b")
			if command == "ZZ" {
				send(t, d.Session, command)
				eventually(t, "ZZ filename prompt", func() bool { return strings.Contains(ansi.Strip(d.Terminal.Snapshot().Content), "Save score as:") })
			} else {
				d.prompt(t, command)
			}
			send(t, d.Session, "\r")
			d.adopt(t, filepath.Join(d.dir, "starlit-cadenza.ly"))
			eventually(t, "successful first save completes quit", func() bool {
				select {
				case <-d.Done():
					return true
				default:
					return false
				}
			})
			if err := d.Err(); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("other buffer", func(t *testing.T) {
		d := startDraftVim(t)
		other := filepath.Join(d.dir, "other.ly")
		send(t, d.Session, ":edit "+other+"\ric8\x1b:w\r")
		eventually(t, "other buffer uses normal save", func() bool { return readEquals(other, "c8\n") })
		for _, marker := range []string{d.source, d.saved} {
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("other buffer was adopted or notified")
			}
		}
		send(t, d.Session, ":buffer 1\ric4\x1b")
		d.prompt(t, ":w")
		send(t, d.Session, "\r")
		d.adopt(t, filepath.Join(d.dir, "starlit-cadenza.ly"))
	})
}
