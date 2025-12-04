package fs

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestSanitize(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "It should remove all the default characters",
			input:    "a_file_name`'$&(){}[];#%^!+=",
			expected: "a_file_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDefaultChars(tt.input)
			assert.Equal(t, got, tt.expected)
		})
	}

}
