package main

import (
	"fmt"
	"strings"

	"github.com/MSmaili/rnm/internal/common"
	"github.com/MSmaili/rnm/internal/fs"
	"github.com/MSmaili/rnm/internal/history"
	"github.com/MSmaili/rnm/internal/log"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo rename operations",
	Long:  `Undo rename operations from history.`,
	RunE:  runUndo,
	Example: `  # Undo most recent operation in current directory
  rnm undo
  `,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	dryRun := globalCfg.DryRun

	adapter := fs.NewAdapter()
	store, err := history.NewGlobalStore(adapter)
	if err != nil {
		return fmt.Errorf("failed to initialize history store: %w", err)
	}

	dirPath := "."

	entry, err := store.Latest(dirPath)
	if err != nil {
		return err
	}

	err = fs.Apply(mapHistoryInReverseToFs(entry), dryRun)
	if err != nil {
		return fmt.Errorf("rename operation failed: %w", err)
	}

	separator := strings.Repeat("=", 60)
	log.Info("%s\n", separator)
	log.Info("  ✓ UNDO COMPLETED SUCCESSFULLY\n")
	log.Info("%s\n", separator)

	if dryRun {
		log.Info("We would have removed entry from history\n")
		return nil
	}

	err = store.Delete(dirPath)
	if err != nil {
		return err
	}
	log.Info("We removed the entry from history\n")

	return nil
}

func mapHistoryInReverseToFs(entry *history.Entry) []fs.RenameOp {
	return common.MapSlice(entry.Operations, func(e history.Operation) fs.RenameOp {
		return fs.RenameOp{
			OldPath: e.New,
			NewPath: e.Old,
		}
	})
}
