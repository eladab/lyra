package watcher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Event struct {
	Changed bool
	Files   int
	Warning string
}

// Run watches containing directories, surviving Vim's atomic-save renames. A
// periodic fingerprint scan also recovers from lost events, replaced parents,
// symlink targets and filesystems without native notifications. notify must not
// block; compilation and debounce belong to the caller.
func Run(ctx context.Context, source, notification string, includeDirs []string, notify func(Event)) {
	w, err := fsnotify.NewWatcher()
	var events <-chan fsnotify.Event
	var faults <-chan error
	baseWarning := ""
	if err == nil {
		defer w.Close()
		events, faults = w.Events, w.Errors
	} else {
		baseWarning = "File notifications unavailable; polling once per second"
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var previous graph
	watched := make(map[string]bool)
	refresh := func() {
		next := discover(source, notification, includeDirs)
		if w != nil {
			dirs := make(map[string]bool)
			for path := range next.files {
				dir := filepath.Dir(path)
				for {
					if info, err := os.Stat(dir); err == nil && info.IsDir() {
						break
					}
					parent := filepath.Dir(dir)
					if parent == dir {
						break
					}
					dir = parent
				}
				dirs[dir] = true
			}
			// WatchList reflects watches the kernel dropped on directory removal.
			watched = make(map[string]bool)
			for _, dir := range w.WatchList() {
				watched[dir] = true
			}
			for dir := range watched {
				if !dirs[dir] {
					_ = w.Remove(dir)
				}
			}
			for dir := range dirs {
				if !watched[dir] {
					if err := w.Add(dir); err != nil {
						baseWarning = "Some directory watches unavailable; polling once per second"
					}
				}
			}
		}
		warning := next.warning
		if baseWarning != "" {
			warning = baseWarning + ". " + warning
		}
		if !reflect.DeepEqual(previous.files, next.files) || previous.warning != warning {
			notify(Event{Changed: !reflect.DeepEqual(previous.files, next.files), Files: len(next.files), Warning: warning})
		}
		next.warning = warning
		previous = next
	}
	refresh() // the initial event starts the first build after establishing watches
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if _, tracked := previous.files[event.Name]; tracked || event.Has(fsnotify.Create|fsnotify.Rename|fsnotify.Remove) {
				refresh()
			}
		case _, ok := <-faults:
			if !ok {
				faults = nil
			} else {
				baseWarning = "File notification error; polling once per second"
				refresh()
			}
		case <-ticker.C:
			refresh()
		}
	}
}
