//go:build !ios

package osvfs

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"
)

// We do this right at startup to minimize the chance that executable gets moved or deleted.
var isFileSystemCaseSensitive = func() bool {
	// win32/win64 are case insensitive platforms
	if runtime.GOOS == "windows" {
		return false
	}

	if runtime.GOARCH == "wasm" {
		// !!! Who knows; this depends on the host implementation.
		return true
	}

	// As a proxy for case-insensitivity, we check if the current executable exists under a different case.
	// This is not entirely correct, since different OSs can have differing case sensitivity in different paths,
	// but this is largely good enough for our purposes (and what sys.ts used to do with __filename).
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("vfs: failed to get executable path: %v", err))
	}

	// If the current executable exists under a different case, we must be case-insensitive.
	swapped := swapCase(exe)
	if _, err := os.Stat(swapped); err != nil {
		if os.IsNotExist(err) {
			return true
		}
		panic(fmt.Sprintf("vfs: failed to stat %q: %v", swapped, err))
	}
	return false
}()

// Convert all lowercase chars to uppercase, and vice-versa
func swapCase(str string) string {
	return strings.Map(func(r rune) rune {
		upper := unicode.ToUpper(r)
		if upper == r {
			return unicode.ToLower(r)
		} else {
			return upper
		}
	}, str)
}