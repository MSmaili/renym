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
	Short: "Undo the latest verified rename or organization run",
	Long:  `Undo verified operations from history. New arrivals, changed files and uncertain journals are never replayed automatically.`,
	Args:  cobra.NoArgs,
	RunE:  runUndo,
	Example: `  # Undo most recent operation in current directory
  renym undo
  renym undo --path ~/Downloads --dry-run
  renym undo --run <id>
  `,
}

var undoPath, undoRunID string

func init() {
	rootCmd.AddCommand(undoCmd)
	undoCmd.Flags().StringVarP(&undoPath, "path", "p", ".", "Original input folder to undo")
	undoCmd.Flags().StringVar(&undoRunID, "run", "", "Run ID from renym history (must be the latest eligible run for its input folder)")
	undoCmd.MarkFlagsMutuallyExclusive("path", "run")
}

func runUndo(cmd *cobra.Command, args []string) error {
	if err := validateUndoSelection(cmd); err != nil {
		return err
	}
	dryRun := globalCfg.DryRun

	adapter := fs.NewAdapter()
	store, err := history.NewGlobalStore(adapter)
	if err != nil {
		return fmt.Errorf("failed to initialize history store: %w", err)
	}

	service := app.NewService(adapter, store)
	var result app.Result
	if undoRunID != "" {
		result, err = service.UndoRun(cmd.Context(), undoRunID, dryRun)
	} else {
		result, err = service.Undo(cmd.Context(), undoPath, dryRun)
	}
	if result.Organization && !dryRun {
		printOrganizationOutcome(result, err != nil)
	}
	if err != nil {
		return applicationFailure("undo", result, err)
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
		printUndoPreview(result)
		return nil
	}

	if result.Organization {
		log.Info("Completed undo audit retained in history.\n")
	} else {
		log.Info("We removed the entry from history\n")
	}

	return nil
}

func validateUndoSelection(cmd *cobra.Command) error {
	if cmd.Flags().Changed("run") && undoRunID == "" {
		return fmt.Errorf("--run requires a run ID")
	}
	if cmd.Flags().Changed("path") && undoPath == "" {
		return fmt.Errorf("--path requires an input folder")
	}
	return nil
}

func printUndoPreview(result app.Result) {
	for _, op := range result.Plan.Operations {
		log.Info("Would %s: %s -> %s\n", operationAction(op.OldPath, op.NewPath), op.OldPath, op.NewPath)
	}
	for _, directory := range result.DirectoryCleanupCandidates {
		log.Info("Would check owned directory for empty cleanup: %s\n", directory)
	}
	if result.Organization {
		log.Info("Would retain the completed undo audit; non-empty or changed directories are preserved.\n")
	} else {
		log.Info("We would have removed entry from history\n")
	}
}
