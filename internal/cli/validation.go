package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

var ValidModes = []string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title", "screaming"}

// ErrConflictingFlags is returned when mutually exclusive flags are used together
var ErrConflictingFlags = errors.New("conflicting flags")

func ValidateMode(mode string) error {
	if slices.Contains(ValidModes, mode) {
		return nil
	}
	return fmt.Errorf("invalid mode '%s'. Valid modes are: %s", mode, strings.Join(ValidModes, ", "))
}

func ValidatePath(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("cannot access path: %w", err)
	}
	return nil
}

func ValidateFlags(mode, path string) error {
	if err := ValidateMode(mode); err != nil {
		return err
	}

	if err := ValidatePath(path); err != nil {
		return err
	}

	return nil
}

// ValidateGlobalFlags validates global/persistent flags for conflicts.
// Returns an error if mutually exclusive flags are used together.
func ValidateGlobalFlags(verbose, quiet bool) error {
	if verbose && quiet {
		return fmt.Errorf("%w: --verbose and --quiet cannot be used together", ErrConflictingFlags)
	}
	return nil
}
