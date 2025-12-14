//go:build windows

package fs

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestBaseNameWithoutExt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"filename with extension", "file.txt", "FILE"},
		{"filename without extension", "file", "FILE"},
		{"filename with multiple dots", "file.tar.gz", "FILE"},
		{"converts to uppercase", "lowercase.txt", "LOWERCASE"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := baseNameWithoutExt(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestWindowsFSAdapterIsValidName(t *testing.T) {
	adapter := WindowsFSAdapter{}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid filename", "document.txt", true},
		{"valid filename with spaces", "my document.txt", true},
		{"valid filename with multiple dots", "archive.tar.gz", true},

		{"empty string", "", false},
		{"single dot", ".", false},
		{"double dot", "..", false},

		{"contains <", "file<name.txt", false},
		{"contains >", "file>name.txt", false},
		{"contains :", "file:name.txt", false},
		{"contains \"", "file\"name.txt", false},
		{"contains /", "file/name.txt", false},
		{"contains |", "file|name.txt", false},
		{"contains ?", "file?name.txt", false},
		{"contains *", "file*name.txt", false},
		{"contains many invalid chars", "file<>:|?.txt", false},

		{"CON", "CON", false},
		{"PRN", "PRN", false},
		{"AUX", "AUX", false},
		{"NUL", "NUL", false},
		{"COM1", "COM1", false},
		{"COM5", "COM5", false},
		{"COM9", "COM9", false},
		{"LPT1", "LPT1", false},
		{"LPT5", "LPT5", false},
		{"LPT9", "LPT9", false},

		{"CON.txt", "CON.txt", false},
		{"PRN.doc", "PRN.doc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.IsValidName(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestWindowsFSAdapter_SanitizeName(t *testing.T) {
	adapter := WindowsFSAdapter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid filename unchanged", "document.txt", "document.txt"},
		{"valid filename with spaces unchanged", "my document.txt", "my document.txt"},

		{"replace <", "file<name.txt", "file_name.txt"},
		{"replace >", "file>name.txt", "file_name.txt"},
		{"replace :", "file:name.txt", "file_name.txt"},
		{"replace \"", "file\"name.txt", "file_name.txt"},
		{"replace /", "file/name.txt", "file_name.txt"},
		{"replace |", "file|name.txt", "file_name.txt"},
		{"replace ?", "file?name.txt", "file_name.txt"},
		{"replace *", "file*name.txt", "file_name.txt"},
		{"replace multiple invalid chars", "file<>:|?.txt", "file_____.txt"},
		{"replace all invalid chars", "<>:\"/\\|?*", "_________"},

		{"trim trailing spaces", "filename   ", "filename"},
		{"trim trailing dots", "filename...", "filename"},
		{"trim mixed end", "filename. . ", "filename"},
		{"trim only from end", "file name.txt  ", "file name.txt"},

		{"prefix CON", "CON", "_CON"},
		{"prefix PRN", "PRN", "_PRN"},
		{"prefix AUX", "AUX", "_AUX"},
		{"prefix NUL", "NUL", "_NUL"},
		{"prefix COM1", "COM1", "_COM1"},
		{"prefix LPT1", "LPT1", "_LPT1"},
		{"prefix reserved with extension", "CON.txt", "_CON.txt"},
		{"prefix lowercase reserved", "con", "_con"},
		{"prefix com1", "com1", "_com1"},
		{"prefix mixed case reserved", "CoM1.doc", "_CoM1.doc"},

		{"sanitize + prefix reserved", "CON<file>.txt", "_CON_file_.txt"},
		{"sanitize, trim, then prefix", "PRN|test. ", "_PRN_test"},
		{"valid name", "CONTAINER", "CONTAINER"},
		{"not reserved similar name", "CONSOLE", "CONSOLE"},

		{"empty string replaces with _", "", "_"},
		{"removes the default characters", "$file-&(1)", "file-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.SanitizeName(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}
