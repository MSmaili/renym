package main

import (
	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/log"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List saved runs, recovery state and input folders",
	Args:  cobra.NoArgs,
	RunE:  runHistory,
}

func init() { rootCmd.AddCommand(historyCmd) }

func runHistory(cmd *cobra.Command, _ []string) error {
	adapter := fs.NewAdapter()
	store, err := history.NewGlobalStore(adapter)
	if err != nil {
		return err
	}
	runs, err := app.NewService(adapter, store).Runs(cmd.Context())
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		log.Info("No saved runs.\n")
		return nil
	}
	for _, run := range runs {
		printHistoryRun(run)
	}
	log.Info("Undo the latest eligible run: renym undo --run <id>\n")
	return nil
}

func printHistoryRun(run history.Run) {
	if run.Error != "" {
		log.Info("%s  corrupt (%s)\n", run.ID, run.Error)
		return
	}
	log.Info("%s  %s  schema=%d\n", run.ID, run.State, run.SchemaVersion)
	log.Info("  Input: %s\n  Recorded steps: %d, undone: %d; directories: %d owned, %d processed, %d retained\n", run.Path, run.Completed, run.Undone, run.Directories, run.Cleaned, run.RetainedDirectories)
	if run.ActiveAction != "" {
		log.Info("  Active intent: %s\n", run.ActiveAction)
	}
	if run.RequiresReconciliation {
		log.Info("  Reconciliation required; automatic undo is blocked.\n")
	}
}
