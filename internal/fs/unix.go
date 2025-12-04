//go:build !windows

package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type UnixFSAdapter struct {
	caseSensitive     bool
	caseSensitiveOnce sync.Once
}

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
	a.caseSensitiveOnce.Do(func() {
		a.caseSensitive = detectCaseSensitivity()
	})
	return a.caseSensitive
}

func detectCaseSensitivity() bool {
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "rnm_case_test_UPPER.tmp")

	if err := os.Remove(testFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return true
	}

	if err := os.WriteFile(testFile, []byte{}, 0600); err != nil {
		return true
	}
	defer os.Remove(testFile)

	lowerFile := filepath.Join(tmpDir, "rnm_case_test_upper.tmp")
	_, err := os.Stat(lowerFile)
	return err != nil // if Stat fails → case sensitive
}
