//go:build windows

package fs

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/assert"
)

func TestWindowsFSAdapterIsValidName(t *testing.T) {
	adapter := WindowsFSAdapter{}

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "empty string is invalid",
			test: func(t *testing.T) {
				assert.False(t, adapter.IsValidName(""), "empty string should be invalid")
			},
		},
		{
			name: "reserved name CON",
			test: func(t *testing.T) {
				assert.True(t, adapter.IsValidName("CON"), "CON should be a reserved name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}

func TestWindowsFSAdapterSanitizeName(t *testing.T) {
	adapter := WindowsFSAdapter{}

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "replaces less than sign",
			test: func(t *testing.T) {
				result := adapter.SanitizeName("file<name.txt")
				assert.Equal(t, result, "file_name.txt")
			},
		},
		{
			name: "prefixes reserved name CON",
			test: func(t *testing.T) {
				result := adapter.SanitizeName("CON")
				assert.Equal(t, result, "_CON")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}
