package fs

import (
	"fmt"
	"os"
)

type FileSystemAdapter interface {
	IsValidName(name string) bool
	SanitizeName(name string) string
	IsCaseSensitive() bool
}

type RenameOp struct {
	OldPath string
	NewPath string
}

func Apply(ops []RenameOp, dryRun bool) error {
	for _, op := range ops {
		if dryRun {
			fmt.Printf("Would rename: %s -> %s\n", op.OldPath, op.NewPath)
		} else {
			if err := os.Rename(op.OldPath, op.NewPath); err != nil {
				return fmt.Errorf("failed to rename %s to %s: %w", op.OldPath, op.NewPath, err)
			}
			//TODO: a mode to log on success?
		}
	}
	return nil
}
