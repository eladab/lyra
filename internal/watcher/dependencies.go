// Package watcher tracks the launch score and literal project includes.
package watcher

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type fingerprint struct {
	Size, Modified int64
	Hash           [32]byte
	Error          string
}

type graph struct {
	files   map[string]fingerprint
	warning string
}

func read(path string) ([]byte, fingerprint) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fingerprint{Error: err.Error()}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fingerprint{Error: "not a readable regular file"}
	}
	data, err := io.ReadAll(io.LimitReader(f, 4*1024*1024+1))
	state := fingerprint{Size: info.Size(), Modified: info.ModTime().UnixNano(), Hash: sha256.Sum256(data)}
	if err != nil {
		state.Error = err.Error()
	} else if len(data) > 4*1024*1024 {
		state.Error = "include scanning limited to 4 MiB per file"
		data = nil
	}
	return data, state
}

// literalIncludes skips comments and quoted strings instead of matching include
// examples in lyrics/comments. Scheme-computed dependencies require rebuilding
// manually; the scanner deliberately never evaluates source code.
func literalIncludes(data []byte) (names []string, dynamic bool) {
	s := string(data)
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "%{"):
			depth := 1
			i += 2
			for i < len(s) && depth > 0 {
				if strings.HasPrefix(s[i:], "%{") {
					depth++
					i += 2
				} else if strings.HasPrefix(s[i:], "%}") {
					depth--
					i += 2
				} else {
					i++
				}
			}
		case s[i] == '%':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case s[i] == '"':
			_, i = quoted(s, i)
		case strings.HasPrefix(s[i:], `\include`) && (i+8 == len(s) || strings.ContainsRune(" \t\r\n\"", rune(s[i+8]))):
			i += 8
			for i < len(s) && strings.ContainsRune(" \t\r\n", rune(s[i])) {
				i++
			}
			if i < len(s) && s[i] == '"' {
				var name string
				name, i = quoted(s, i)
				if name != "" {
					names = append(names, name)
				}
			} else {
				dynamic = true
			}
		default:
			i++
		}
	}
	return
}

func quoted(s string, start int) (string, int) {
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if s[i] == '"' {
			v, err := strconv.Unquote(s[start : i+1])
			if err == nil {
				return v, i + 1
			}
			return "", i + 1
		}
	}
	return "", len(s)
}

func discover(source, notification string, includeDirs []string) graph {
	g := graph{files: make(map[string]fingerprint)}
	visited := make(map[string]bool)
	remaining := 32 * 1024 * 1024
	var visit func(string)
	visit = func(path string) {
		path = filepath.Clean(path)
		if visited[path] {
			return
		}
		if len(g.files) >= 512 || remaining <= 0 {
			g.warning = "Include scan limit reached; use manual rebuild for other dependencies"
			return
		}
		visited[path] = true
		data, state := read(path)
		remaining -= len(data)
		g.files[path] = state
		if len(data) == 0 {
			if state.Error != "" && !strings.Contains(state.Error, "no such file") {
				g.warning = fmt.Sprintf("Cannot scan %s: %s", filepath.Base(path), state.Error)
			}
			return
		}
		names, dynamic := literalIncludes(data)
		if dynamic {
			g.warning = "Computed includes require manual rebuild (Ctrl+\\ b)"
		}
		for _, name := range names {
			if len(g.files) >= 512 || remaining <= 0 {
				g.warning = "Include scan limit reached; use manual rebuild for other dependencies"
				break
			}
			if filepath.IsAbs(name) {
				visit(name)
				continue
			}
			// LilyPond's relative-includes mode resolves nested project includes
			// against the including file; -I directories are explicit fallbacks.
			dirs := append([]string{filepath.Dir(path), filepath.Dir(source)}, includeDirs...)
			for _, dir := range dirs {
				if len(g.files) >= 512 {
					break
				}
				candidate := filepath.Join(dir, name)
				_, state := read(candidate)
				g.files[candidate] = state // also notice a previously missing include
				if state.Error == "" {
					visit(candidate)
					break
				}
			}
		}
	}
	visit(source)
	if notification != "" {
		_, g.files[notification] = read(notification)
	}
	return g
}
