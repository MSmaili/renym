package fs

import "strings"

// sanitizeDefaultChars removes extra not desired characters from filenames
func sanitizeDefaultChars(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '`', '\'', '$', '&', '(', ')', '{', '}', '[', ']', ';', '#', '%', '^', '!', '+', '=':
			return -1
		}
		return r
	}, name)
}
