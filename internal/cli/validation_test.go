package cli

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestValidateMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		expectErr bool
	}{
		{"valid_upper", "upper", false},
		{"valid_lower", "lower", false},
		{"valid_pascal", "pascal", false},
		{"valid_camel", "camel", false},
		{"valid_snake", "snake", false},
		{"valid_kebab", "kebab", false},
		{"valid_title", "title", false},
		{"valid_screaming", "screaming", false},
		{"invalid_mode", "invalid", true},
		{"empty_mode", "", true},
		{"random_mode", "randomstring", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMode(tt.mode)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{"valid_temp_dir", tempDir, false},
		{"valid_current_dir", ".", false},
		{"invalid_nonexistent", "/nonexistent/path/that/does/not/exist", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestValidateFlags(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		mode      string
		path      string
		expectErr bool
	}{
		{"valid_mode_and_path", "snake", tempDir, false},
		{"invalid_mode", "invalid", tempDir, true},
		{"invalid_path", "snake", "/nonexistent", true},
		{"both_invalid", "invalid", "/nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.mode, tt.path)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestValidateGlobalFlags(t *testing.T) {
	tests := []struct {
		name      string
		verbose   bool
		quiet     bool
		expectErr bool
	}{
		{"neither_flag", false, false, false},
		{"verbose_only", true, false, false},
		{"quiet_only", false, true, false},
		{"both_flags_conflict", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGlobalFlags(tt.verbose, tt.quiet)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
