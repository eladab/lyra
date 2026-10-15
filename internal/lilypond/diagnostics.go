package lilypond

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Diagnostic struct {
	File, Severity, Message string
	Line, Column            int
}

var (
	withColumn = regexp.MustCompile(`^(.*):([0-9]+):([0-9]+):\s*(fatal error|programming error|error|warning|note):\s*(.*)$`)
	withLine   = regexp.MustCompile(`^(.*):([0-9]+):\s*(fatal error|programming error|error|warning|note):\s*(.*)$`)
	withoutPos = regexp.MustCompile(`^(fatal error|programming error|error|warning|note):\s*(.*)$`)
)

// ParseDiagnostics uses separate patterns so colons in filenames cannot absorb
// the line number. The unmodified, bounded compiler log is retained separately.
func ParseDiagnostics(log, dir string) []Diagnostic {
	var result []Diagnostic
	last := -1
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSuffix(line, "\r")
		var d Diagnostic
		if m := withColumn.FindStringSubmatch(line); m != nil {
			d.File, d.Severity, d.Message = m[1], m[4], m[5]
			d.Line, _ = strconv.Atoi(m[2])
			d.Column, _ = strconv.Atoi(m[3])
		} else if m := withLine.FindStringSubmatch(line); m != nil {
			d.File, d.Severity, d.Message = m[1], m[3], m[4]
			d.Line, _ = strconv.Atoi(m[2])
		} else if m := withoutPos.FindStringSubmatch(line); m != nil {
			d.Severity, d.Message = m[1], m[2]
		} else {
			if last >= 0 && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
				result[last].Message += "\n" + line
			} else {
				last = -1
			}
			continue
		}
		if d.File != "" && !filepath.IsAbs(d.File) {
			d.File = filepath.Join(dir, d.File)
		}
		if len(result) < 200 {
			result = append(result, d)
			last = len(result) - 1
		} else {
			last = -1
		}
	}
	return result
}
