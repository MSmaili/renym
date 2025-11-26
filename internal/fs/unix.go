package fs

import (
	"strings"
)

type UnixFSAdapter struct {
}

func (a *UnixFSAdapter) IsValidName(name string) bool {
	return !strings.Contains(name, "/") && name != "" && name != "." && name != ".."
}

func (a *UnixFSAdapter) SanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

func (a *UnixFSAdapter) IsCaseSensitive() bool {
	return detectCaseSensitivity()
}

func detectCaseSensitivity() bool {
	//TODO: should we add some logic and check if case sensitivity is allowed
	// macos is case insensitive by default but ubuntu is not??
	// we should add some logic here
	return true
}
