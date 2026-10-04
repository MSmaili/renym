//go:build !windows

package fs

import (
	"fmt"
	"strings"
	"syscall"
)

type UnixFSAdapter struct{}

func (a *UnixFSAdapter) IsValidName(name string) bool {
	return name != "" &&
		name != "." &&
		name != ".." &&
		!strings.Contains(name, "/") &&
		strings.IndexByte(name, 0) == -1
}

func (a *UnixFSAdapter) SanitizeName(name string) string {
	result := sanitizeDefaultChars(name)
	result = strings.ReplaceAll(result, "/", "_")
	result = strings.ReplaceAll(result, "\x00", "_")
	return result
}

func (a *UnixFSAdapter) IsCaseSensitive() bool {
	// A temporary-volume probe says nothing about the selected volume. Until
	// per-volume capabilities exist, compare targets conservatively. Lstat and
	// native no-replace execution remain authoritative for actual occupancy.
	return false
}

func (a *UnixFSAdapter) PathIdentifier(path string) (string, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return "", fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}
