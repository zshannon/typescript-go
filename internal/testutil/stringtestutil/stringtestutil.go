package stringtestutil

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/stringutil"
)

func Dedent(text string) string {
	lines := strings.Split(text, "\n")
	// Remove blank lines in the beginning and end
	// and convert all tabs in the beginning of line to spaces
	startLine := -1
	lastLine := 0
	for i, line := range lines {
		firstNonTab := strings.IndexFunc(line, func(r rune) bool {
			return r != '\t'
		})
		if firstNonTab > 0 {
			line = strings.Repeat("    ", firstNonTab) + line[firstNonTab:]
			lines[i] = line
		}
		line = strings.TrimSpace(line)
		if line != "" {
			if startLine == -1 {
				startLine = i
			}
			lastLine = i
		}
	}
	lines = lines[startLine : lastLine+1]
	indentation := stringutil.GuessIndentation(lines)
	if indentation > 0 {
		for i := range lines {
			if len(lines[i]) > indentation {
				lines[i] = lines[i][indentation:]
			} else {
				lines[i] = ""
			}
		}
	}
	return strings.Join(lines, "\n")
}
