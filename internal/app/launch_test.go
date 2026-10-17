//go:build darwin || linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"lyra/internal/scoretemplate"
)

func runLaunchCommand(m *launcher, cmd tea.Cmd) {
	for cmd != nil {
		_, next := m.Update(cmd())
		cmd = next
	}
}

func launchKey(data string) uv.KeyPressEvent {
	var decoder uv.EventDecoder
	_, event := decoder.Decode([]byte(data))
	return event.(uv.KeyPressEvent)
}

func TestScoreBrowserAndSafeCreation(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"a folder", ".hidden"} {
		if err := os.Mkdir(filepath.Join(dir, folder), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"nocturne 雨.ly", "Part.LY", "readme.txt", ".private.ly"} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte("keep me"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "a folder"), filepath.Join(dir, "linked folder")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.ly"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, _, err := scoreDirectory(context.Background(), dir, false)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.name)
	}
	if !reflect.DeepEqual(names, []string{"..", "a folder", "linked folder", "nocturne 雨.ly", "Part.LY"}) {
		t.Fatal(names)
	}
	for _, path := range []string{"readme.txt", "pipe.ly", "absent.ly"} {
		if _, _, err := chooseScorePath(context.Background(), path, dir, false); err == nil {
			t.Fatal("invalid score opened", path)
		}
	}
	opened, directory, err := chooseScorePath(context.Background(), "nocturne 雨.ly", dir, false)
	if err != nil || directory || opened != filepath.Join(dir, "nocturne 雨.ly") {
		t.Fatal(opened, directory, err)
	}
	if _, directory, err := chooseScorePath(context.Background(), "linked folder", dir, false); err != nil || !directory {
		t.Fatal("directory symlink could not be browsed", err)
	}
	for _, value := range []string{"nocturne 雨.ly", "../missing-dir/new.ly", "wrong.txt", "a folder/", "two\npaths.ly"} {
		if _, _, err := chooseScorePath(context.Background(), value, dir, true); err == nil {
			t.Fatal("invalid destination created", value)
		}
	}
	data, _ := os.ReadFile(opened)
	if string(data) != "keep me" {
		t.Fatal("existing score was changed")
	}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		wg.Go(func() { _, _, err := chooseScorePath(context.Background(), "new score 雨", dir, true); results <- err })
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("exclusive creation did not prevent a collision", success)
	}
	if info, err := os.Stat(filepath.Join(dir, "new score 雨.ly")); err != nil || info.Size() != 0 {
		t.Fatal("new score was not created at the chosen path", err)
	}
	t.Setenv("HOME", dir)
	if path, err := scorePath("~/Part.LY", "/"); err != nil || path != filepath.Join(dir, "Part.LY") {
		t.Fatal("home path did not expand", path, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := chooseScorePath(ctx, "cancelled.ly", dir, true); err != context.Canceled {
		t.Fatal("cancelled selection created a score", err)
	}
}

func TestLauncherNavigationFieldsAndStaleResults(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "scores"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scores", "first.ly"), []byte("c4"), 0600); err != nil {
		t.Fatal(err)
	}
	m := newLauncher(context.Background(), nil, dir, 100, 32)
	runLaunchCommand(m, m.key(launchKey("o")))
	if m.page != launchBrowse || len(m.entries) < 2 || !m.entries[1].dir {
		t.Fatal("open chooser did not list folders")
	}
	runLaunchCommand(m, m.key(launchKey("\x1b[B")))
	runLaunchCommand(m, m.key(launchKey("\r")))
	if m.directory != filepath.Join(dir, "scores") {
		t.Fatal("Enter did not browse selected folder")
	}
	m.selected = 1
	runLaunchCommand(m, m.key(launchKey("\r")))
	if m.chosen != filepath.Join(dir, "scores", "first.ly") {
		t.Fatal("Enter did not open the selected .ly score", m.chosen)
	}
	m.chosen = ""
	stale := m.load(dir)
	m.pathField(launchPath, "scores/")
	runLaunchCommand(m, stale)
	if m.page != launchPath || string(m.field) != "scores/" {
		t.Fatal("stale directory result changed the path form")
	}
	m.pathField(launchPath, "scores/fi")
	runLaunchCommand(m, m.complete())
	if string(m.field) != filepath.Join(dir, "scores", "first.ly") {
		t.Fatal("path completion failed", string(m.field))
	}
	m.pathField(launchPath, "scores/fi")
	stale = m.complete()
	m.insert("x")
	runLaunchCommand(m, stale)
	if string(m.field) != "scores/fix" {
		t.Fatal("stale completion overwrote edited text")
	}
	m.pathField(launchCreate, "")
	m.insert("雨 notes.ly")
	m.fieldKey(launchKey("\x1b[D"))
	m.fieldKey(launchKey("\x7f"))
	if string(m.field) != "雨 notes.y" {
		t.Fatal("Unicode cursor/backspace corrupted the filename", string(m.field))
	}
	m.insert("\n\x1b[2J")
	if m.err == "" || strings.ContainsAny(string(m.field), "\x1b\n") {
		t.Fatal("control-bearing paste became commands or field data")
	}
	m.fieldKey(launchKey("\x15"))
	if len(m.field) != 0 || m.cursor != 0 {
		t.Fatal("Ctrl+U did not clear the path")
	}
	runLaunchCommand(m, m.key(launchKey("\x1b")))
	if m.page != launchBrowse || m.chosen != "" {
		t.Fatal("cancelled creation chose a file")
	}
}

func TestLauncherFitsTerminalAndSanitizesPaths(t *testing.T) {
	m := newLauncher(context.Background(), nil, "/scores/雨\x1b[2J/"+strings.Repeat("a long folder name/", 8), 100, 32)
	for _, size := range [][2]int{{1, 1}, {20, 8}, {28, 18}, {60, 24}, {120, 40}} {
		m.width, m.height = size[0], size[1]
		for _, page := range []launchPage{launchMenu, launchBrowse, launchCreate, launchPath, launchTemplate} {
			m.page = page
			m.field, m.cursor = []rune(m.directory), len([]rune(m.directory))
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) > m.height {
				t.Fatal("launcher exceeded terminal rows", size, page)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) != m.width || strings.Contains(ansi.Strip(line), "\x1b") {
					t.Fatal("launcher exceeded columns or leaked path controls", size, page, line)
				}
			}
			if m.width >= 28 && m.height >= 18 {
				top, bottom := -1, -1
				for row, line := range lines {
					plain := strings.TrimSpace(ansi.Strip(line))
					if strings.HasPrefix(plain, "╭") {
						top = row
					}
					if strings.HasPrefix(plain, "╰") {
						bottom = row
					}
				}
				expected := 16
				if page == launchMenu {
					expected = 18
				} else if page == launchBrowse {
					expected = 12 + m.listRows()
				} else if page == launchTemplate {
					expected = 14 + m.templateRows()
				}
				if top < 0 || bottom-top+1 != expected {
					t.Fatal("dialog content wrapped or its frame was clipped", size, page, top, bottom)
				}
				for _, line := range lines[top+1 : bottom] {
					plain := strings.TrimSpace(ansi.Strip(line))
					if !strings.HasPrefix(plain, "│") || !strings.HasSuffix(plain, "│") || ansi.StringWidth(plain) != min(78, m.width-4) {
						t.Fatal("dialog row broke its frame", size, page, plain)
					}
				}
			}
		}
	}
}

func TestNewScoreTemplateConfirmationAndBackNavigation(t *testing.T) {
	for _, query := range []string{"", "quartet string"} {
		t.Run(query, func(t *testing.T) {
			dir := t.TempDir()
			m := newLauncher(context.Background(), nil, dir, 100, 32)
			m.creating = true
			m.pathField(launchCreate, "a new score 雨")
			runLaunchCommand(m, m.key(launchKey("\r")))
			path := filepath.Join(dir, "a new score 雨.ly")
			if m.page != launchTemplate || m.destination != path || m.chosen != "" {
				t.Fatal("filename confirmation did not open the template step")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("file was created before template confirmation")
			}
			m.insert("this query has no match xyz")
			runLaunchCommand(m, m.key(launchKey("\r")))
			if m.chosen != "" || len(m.templates) != 0 {
				t.Fatal("unmatched template query created a score")
			}
			runLaunchCommand(m, m.key(launchKey("\x1b")))
			if m.page != launchCreate || string(m.field) != path {
				t.Fatal("Esc did not restore the chosen filename")
			}
			runLaunchCommand(m, m.key(launchKey("\r")))
			m.insert(query)
			want := scoretemplate.Search(query)[0].Code
			runLaunchCommand(m, m.key(launchKey("\r")))
			data, err := os.ReadFile(path)
			if err != nil || string(data) != want || m.chosen != path {
				t.Fatal("new file did not contain the selected template", err)
			}
		})
	}
}

func TestTemplateDestinationRaceAndStaleValidation(t *testing.T) {
	dir := t.TempDir()
	m := newLauncher(context.Background(), nil, dir, 100, 32)
	m.creating = true
	m.pathField(launchCreate, "score.ly")
	stale := m.choose("score.ly", true)
	m.pathField(launchCreate, "changed.ly")
	runLaunchCommand(m, stale)
	if m.page != launchCreate || m.destination != "" {
		t.Fatal("stale path validation changed the template destination")
	}
	m.busy = false
	runLaunchCommand(m, m.choose("score.ly", true))
	path := filepath.Join(dir, "score.ly")
	if err := os.WriteFile(path, []byte("user score"), 0600); err != nil {
		t.Fatal(err)
	}
	m.insert("single staff notes only")
	runLaunchCommand(m, m.key(launchKey("\r")))
	data, _ := os.ReadFile(path)
	if string(data) != "user score" || m.chosen != "" || !strings.Contains(m.err, "already exists") || m.busy {
		t.Fatal("confirmation replaced a destination created while searching")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	cancel()
	runLaunchCommand(m, m.key(launchKey("\r")))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled template creation wrote a file")
	}
}

func TestTemplateSearchEditingAndSelection(t *testing.T) {
	m := newLauncher(context.Background(), nil, t.TempDir(), 100, 32)
	m.pathField(launchTemplate, "")
	m.insert("piano")
	first := m.templates[0].ID
	m.fieldKey(launchKey("\x1b[B"))
	if m.selected != 1 || m.templates[m.selected].ID == first {
		t.Fatal("template arrow selection failed")
	}
	m.fieldKey(launchKey("\x10"))
	if m.selected != 0 {
		t.Fatal("Ctrl+P selection failed")
	}
	m.fieldKey(launchKey("\x15"))
	if len(m.field) != 0 || m.templates[0].ID != "blank" {
		t.Fatal("clearing search did not restore the blank option")
	}
	m.insert("雨")
	m.fieldKey(launchKey("\x7f"))
	if len(m.field) != 0 || len(m.templates) != 23 {
		t.Fatal("Unicode search/backspace was corrupted")
	}
	m.insert("q\n\x1b[2J")
	if m.err == "" || len(m.field) != 0 {
		t.Fatal("control-bearing paste entered the template query")
	}
}
