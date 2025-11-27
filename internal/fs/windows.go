//go:build windows

package fs

import "strings"

type WindowsFSAdapter struct{}

var (
	windowsInvalidChars  = []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	windowsReservedNames = map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
		"COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
		"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
)

func baseNameWithoutExt(name string) string {
	name = strings.ToUpper(name)
	if idx := strings.Index(name, "."); idx != -1 {
		return name[:idx]
	}
	return name
}

func (a WindowsFSAdapter) IsValidName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	for _, char := range windowsInvalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	return windowsReservedNames[baseNameWithoutExt(name)]
}

func (a WindowsFSAdapter) SanitizeName(name string) string {
	result := name

	for _, char := range windowsInvalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	result = strings.TrimRight(result, " .")

	if windowsReservedNames[baseNameWithoutExt(result)] {
		result = "_" + result
	}

	return result
}

func (a WindowsFSAdapter) IsCaseSensitive() bool {
	return false
}
