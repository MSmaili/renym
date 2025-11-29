package fs

import (
	"regexp"
	"strings"
)

type WindowsFSAdapter struct{}

var (
	invalidRunes    = `<>:"/\|?*`
	reservedPattern = regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:$|[^A-Za-z0-9])`)
)

func baseNameWithoutExt(name string) string {
	name = strings.ToUpper(name)
	if idx := strings.Index(name, "."); idx != -1 {
		return name[:idx]
	}
	return name
}

func isReservedName(name string) bool {
	base := baseNameWithoutExt(name)
	return reservedPattern.MatchString(base)
}

func sanitizeRune(r rune) rune {
	if r < 32 || (r >= 0xD800 && r <= 0xDFFF) {
		return '_'
	}
	if strings.ContainsRune(invalidRunes, r) {
		return '_'
	}
	return r
}

func (a WindowsFSAdapter) IsValidName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	if strings.ContainsAny(name, invalidRunes) {
		return false
	}

	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return false
	}

	for _, r := range name {
		if r < 32 {
			return false
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return false
		}
	}

	return !isReservedName(name)
}

func (a WindowsFSAdapter) SanitizeName(name string) string {
	result := strings.Map(sanitizeRune, name)
	result = strings.TrimRight(result, " .")
	if result == "" || isReservedName(result) {
		result = "_" + result
	}
	return result
}

func (a WindowsFSAdapter) IsCaseSensitive() bool {
	return false
}
