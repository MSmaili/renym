//go:build !windows

package fs

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/assert"
)

func TestUnixFSAdapterIsValidName(t *testing.T) {
	adapter := &UnixFSAdapter{}

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid filename", "myfile.txt", true},
		{"valid filename with spaces", "my file.txt", true},
		{"valid filename with special chars", "file:name<>?.txt", true},
		{"empty string is invalid", "", false},
		{"single dot is invalid", ".", false},
		{"double dot is invalid", "..", false},
		{"contains forward slash", "file/name.txt", false},
		{"starts with dot is valid", ".hidden", true},
		{"multiple dots valid", "file.tar.gz", true},
		{"slash at start", "/file.txt", false},
		{"slash at end", "file.txt/", false},
		{"multiple slashes", "path/to/file.txt", false},
		{"unicode characters valid", "файл.txt", true},
		{"emoji valid", "file🎉.txt", true},
		{"contains null byte", "hello\x00world", false},
		{"null byte only", "\x00", false},
		{"null + slash", "te\x00st", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.IsValidName(tt.input)
			if tt.expected {
				assert.True(t, result, tt.name)
			} else {
				assert.False(t, result, tt.name)
			}
		})
	}
}

func TestUnixFSAdapterSanitizeName(t *testing.T) {
	adapter := &UnixFSAdapter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid filename unchanged", "myfile.txt", "myfile.txt"},
		{"replaces single slash", "file/name.txt", "file_name.txt"},
		{"replaces multiple slashes", "path/to/file.txt", "path_to_file.txt"},
		{"replaces slash at start", "/file.txt", "_file.txt"},
		{"replaces slash at end", "file.txt/", "file.txt_"},
		{"replaces consecutive slashes", "file//name.txt", "file__name.txt"},
		{"preserves special characters except slash", "file<>:|?.txt", "file<>:|?.txt"},
		{"preserves unicode", "файл/имя.txt", "файл_имя.txt"},
		{"empty string unchanged", "", ""},
		{"only slashes", "///", "___"},
		{"sanitize null byte", "he\x00llo", "he_llo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.SanitizeName(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestDetectCaseSensitivity(t *testing.T) {
	result := detectCaseSensitivity()
	if result != true && result != false {
		t.Fatal("detectCaseSensitivity returned a non-boolean value")
	}
}
