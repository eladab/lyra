//go:build darwin || linux

package editor

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed lilypond.vim
var lilyPondSyntax []byte

// The overlay lives only for this child session. It is sourced with -S after
// Vim has read the user's startup files and plugins; HOME is never modified.
func writeSyntaxOverlay() (dir, script string, err error) {
	dir, err = os.MkdirTemp("", "lyra-vim-")
	if err != nil {
		return "", "", fmt.Errorf("create Vim syntax directory: %w", err)
	}
	script = filepath.Join(dir, "lilypond.vim")
	if err = os.WriteFile(script, lilyPondSyntax, 0600); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", fmt.Errorf("create Vim syntax layer: %w", err)
	}
	return dir, script, nil
}
