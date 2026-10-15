package watcher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLiteralIncludesAndDependencyGraph(t *testing.T) {
	text := `% \include "comment.ily"
%{ \include "block.ily" %{ nested %} %}
title = "ignore \\include \"string.ily\""
\include "parts/notes.ily"
\include variable
`
	names, dynamic := literalIncludes([]byte(text))
	if !dynamic || !reflect.DeepEqual(names, []string{"parts/notes.ily"}) {
		t.Fatalf("%q dynamic=%v", names, dynamic)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "parts"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "score.ly")
	for path, data := range map[string]string{file: text, filepath.Join(dir, "parts/notes.ily"): `\include "../score.ly"` + "\n" + `\include "missing.ily"`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	g := discover(file, "", nil)
	for _, path := range []string{file, filepath.Join(dir, "parts/notes.ily"), filepath.Join(dir, "parts/missing.ily")} {
		if _, ok := g.files[path]; !ok {
			t.Fatalf("dependency not tracked: %s", path)
		}
	}
	if len(g.files) > 5 || g.warning == "" {
		t.Fatalf("cycle or dynamic policy failed: %+v", g)
	}
}

func TestWatcherMissingParentCreationAndReplacement(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "score.ly")
	if err := os.WriteFile(file, []byte(`\include "new/notes.ily"`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	events := make(chan Event, 10)
	go func() {
		defer close(done)
		Run(ctx, file, "", nil, func(e Event) {
			select {
			case events <- e:
			default:
			}
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	wait := func() {
		t.Helper()
		select {
		case event := <-events:
			if !event.Changed {
				t.Fatal(event)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("file change not detected")
		}
	}
	wait()
	if err := os.Mkdir(filepath.Join(dir, "new"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new/notes.ily"), []byte("notes"), 0600); err != nil {
		t.Fatal(err)
	}
	wait()
	if err := os.Rename(filepath.Join(dir, "new"), filepath.Join(dir, "old")); err != nil {
		t.Fatal(err)
	}
	wait()
	if err := os.Mkdir(filepath.Join(dir, "new"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new/notes.ily"), []byte("new notes"), 0600); err != nil {
		t.Fatal(err)
	}
	wait()
}
