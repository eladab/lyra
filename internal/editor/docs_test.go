//go:build darwin || linux

package editor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func docsVim(t *testing.T, rc string, disabled bool) (*Session, string, string) {
	t.Helper()
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("traditional Vim is required for documentation integration")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "score with spaces.ly")
	for name, contents := range map[string]string{
		".vimrc":               "set nocompatible noswapfile viminfo= nomore\nset shortmess+=I\n" + rc + "\nautocmd VimEnter * call writefile(['ready'], '" + filepath.Join(dir, "ready") + "')\n",
		"score with spaces.ly": "\\tuplet 3/2 { c8 d e }\n",
		"include.ily":          "\\relative { c'4 d }\n",
		"other.txt":            "ordinary text\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "HOME" && key != "VIMINIT" && key != "EXINIT" && key != "GVIMINIT" {
			env = append(env, item)
		}
	}
	notification := filepath.Join(dir, "lookup.json")
	s, err := Start(Options{File: file, Env: append(env, "HOME="+dir), Cols: 80, Rows: 24, DocsNotification: notification, DisableDocsKey: disabled})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	eventually(t, "documentation Vim startup", func() bool { return readEquals(filepath.Join(dir, "ready"), "ready\n") })
	return s, dir, notification
}

func TestRealVimKDocumentationAndIncludeBuffers(t *testing.T) {
	s, dir, notification := docsVim(t, "", false)
	request := func(want string) {
		t.Helper()
		eventually(t, "Vim documentation request for "+want, func() bool {
			data, _ := os.ReadFile(notification)
			var request []string
			return json.Unmarshal(data, &request) == nil && len(request) == 2 && request[1] == want
		})
	}
	send(t, s, "gg0lK")
	request("tuplet")
	send(t, s, ":edit "+filepath.Join(dir, "include.ily")+"\rgg0lK")
	request("relative")
	send(t, s, ":LyraDocs key signature\r")
	request("key signature")
	send(t, s, ":edit "+filepath.Join(dir, "other.txt")+"\r")
	values := vimValues(t, s, dir, "[&l:keywordprg, maparg('K', 'n'), string(&modified)]")
	if strings.Join(values, "|") != "||0" {
		t.Fatal("non-LilyPond keyword program, mapping or buffer changed", values)
	}
}

func TestDocumentationPreservesCustomKAndKeywordProgram(t *testing.T) {
	for _, rc := range []string{"nnoremap K :let g:custom_lookup = 1<CR>", "set keywordprg=:echo", "autocmd BufRead *.ly setlocal keywordprg=:echo"} {
		t.Run(rc, func(t *testing.T) {
			s, dir, _ := docsVim(t, rc, false)
			if strings.HasPrefix(rc, "nnoremap") {
				send(t, s, "K")
				if got := vimValues(t, s, dir, "[string(g:custom_lookup)]"); got[0] != "1" {
					t.Fatal("custom K was replaced", got)
				}
			} else if got := vimValues(t, s, dir, "[&keywordprg]"); got[0] != ":echo" {
				t.Fatal("custom keyword program was replaced", got)
			}
		})
	}
	sHelp, dirHelp, _ := docsVim(t, "set keywordprg=", false)
	// Some Vim builds canonicalize an explicitly empty keywordprg to :help.
	if got := vimValues(t, sHelp, dirHelp, "[&keywordprg]"); got[0] != "" && got[0] != ":help" {
		t.Fatal("user's empty keyword program (:help) was replaced", got)
	}
	s, dir, _ := docsVim(t, "", true)
	if got := vimValues(t, s, dir, "[&l:keywordprg, string(exists(':LyraDocs'))]"); strings.Join(got, "|") != "|2" {
		t.Fatal("docs-key opt out did not retain explicit lookup", got)
	}
}
