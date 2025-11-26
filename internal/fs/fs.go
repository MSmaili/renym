package fs

import (
	"fmt"
	"os"
)

type FileSystemAdapter interface {
	IsValidName(name string) bool    // we need a way to tell if file name is valid
	SanitizeName(name string) string // we need a way to sanatize name
	IsCaseSensitive() bool           // we need a way to tell if naming IsCaseSensitive, is different per os
}

type RenameOp struct {
	OldPath string
	NewPath string
}

func Apply(ops []RenameOp) error {
	for _, op := range ops {
		if err := os.Rename(op.OldPath, op.NewPath); err != nil {
			return fmt.Errorf("failed to rename %s to %s: %w", op.OldPath, op.NewPath, err)
		}
		fmt.Printf("Renamed: %s -> %s\n", op.OldPath, op.NewPath)
	}
	return nil
}
