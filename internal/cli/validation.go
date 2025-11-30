package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

var ValidModes = []string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title", "screaming"}

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
