//go:build darwin || linux

package editor

import _ "embed"

//go:embed save.vim
var saveRuntime []byte
