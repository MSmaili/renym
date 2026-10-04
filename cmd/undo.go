package main

import (
	"fmt"
	"strings"

	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/log"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo rename operations",
	Long:  `Undo rename operations from history.`,
	RunE:  runUndo,
	Example: `  # Undo most recent operation in current directory
  renym undo
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

	result, err := app.NewService(adapter, store).Undo(cmd.Context(), dirPath, dryRun)
	if err != nil {
		return fmt.Errorf("undo failed after %d completed operation(s): %w", len(result.Execution.Completed), err)
	}

	separator := strings.Repeat("=", 60)
	log.Info("%s\n", separator)
	if dryRun {
		log.Info("  UNDO PREVIEW - No files were actually renamed\n")
	} else {
		log.Info("  ✓ UNDO COMPLETED SUCCESSFULLY\n")
	}
	log.Info("%s\n", separator)

	if dryRun {
		for _, op := range result.Plan.Operations {
			log.Info("Would rename: %s -> %s\n", op.OldPath, op.NewPath)
		}
		log.Info("We would have removed entry from history\n")
		return nil
	}

	log.Info("We removed the entry from history\n")

	return nil
}
