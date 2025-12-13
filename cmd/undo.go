package main

import (
	"fmt"
	"strings"

	"github.com/MSmaili/rnm/internal/common"
	"github.com/MSmaili/rnm/internal/fs"
	"github.com/MSmaili/rnm/internal/history"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo [history-file]",
	Short: "Undo rename operations",
	Long: `Undo rename operations from history.

Without arguments, undoes the most recent operation.
With a history file path, undoes that specific operation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUndo,
	Example: `  # Undo most recent operation
  rnm undo

  # Undo specific operation
  rnm undo .rnm_history/2025-11-25_230427.json`,
}

func init() {
	rootCmd.AddCommand(undoCmd)
	undoCmd.Flags().BoolP("dry-run", "n", false, "Show what would be renamed without actually renaming")

}

func runUndo(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	maybePath := ""
	if len(args) > 0 {
		maybePath = args[0]
	}

	entry, err := history.Load(maybePath)
	if err != nil {
		return err
	}

	err = fs.Apply(mapHistoryInReverseToFs(entry), dryRun)
	if err != nil {
		return fmt.Errorf("rename operation failed: %w", err)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  ✓ UNDO COMPLETED SUCCESSFULLY")
	fmt.Println(strings.Repeat("=", 60))

	if dryRun {
		fmt.Println("We would have removed entry from history")
		return nil
	}
	err = history.Delete(maybePath)
	if err != nil {
		return err
	}
	fmt.Println("We removed the entry from history")

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
