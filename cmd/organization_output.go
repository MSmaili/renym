package main

import (
	"fmt"
	"path/filepath"

	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/log"
)

func ownedDirectoryPath(directory fs.OwnedDirectory) string {
	return filepath.Join(directory.Root, filepath.FromSlash(directory.Relative))
}

func printOrganizationOutcome(result app.Result, failed bool) {
	printUnverifiedOutcomes(result)
	for _, directory := range result.DirectoriesCreated {
		log.Info("Created directory: %s\n", ownedDirectoryPath(directory))
	}
	for _, directory := range result.DirectoriesRemoved {
		log.Info("Removed owned directory: %s\n", ownedDirectoryPath(directory))
	}
	for _, retained := range result.DirectoriesRetained {
		log.Info("Retained directory: %s (%s)\n", ownedDirectoryPath(retained.Directory), retained.Reason)
	}
	if result.HistoryID != "" {
		if failed {
			log.Error("Run: %s (input: %s)\n", result.HistoryID, result.SourcePath)
		} else {
			log.Info("Run: %s\n", result.HistoryID)
		}
	}
	if result.RequiresReconciliation {
		log.Error("Reconciliation required: do not retry or automatically undo this run. Inspect its retained journal.\n")
	}
}

func printUnverifiedOutcomes(result app.Result) {
	if !result.RequiresReconciliation {
		return
	}
	for _, outcome := range result.MoveOutcomes {
		if outcome.Attempted && !outcome.Verified {
			log.Error("Unverified file mutation: native_completed=%t; not safe to replay.\n", outcome.Completed)
		}
	}
	for _, outcome := range result.DirectoryOutcomes {
		if outcome.Attempted && !outcome.Verified {
			log.Error("Unverified directory mutation: native_completed=%t; not accepted as ownership or cleanup.\n", outcome.Completed)
		}
	}
}

func applicationFailure(action string, result app.Result, err error) error {
	if result.Organization {
		return fmt.Errorf("%s stopped after %d native file completion(s), %d verified directory creation(s) and %d verified directory removal(s)%s: %w", action, len(result.Execution.Completed), len(result.DirectoriesCreated), len(result.DirectoriesRemoved), organizationFailureContext(result), err)
	}
	return fmt.Errorf("%s failed after %d completed operation(s): %w", action, len(result.Execution.Completed), err)
}

func organizationFailureContext(result app.Result) string {
	var context string
	if result.HistoryID != "" {
		context = fmt.Sprintf(" (run %s)", result.HistoryID)
	}
	if result.RequiresReconciliation {
		context += "; reconciliation required: do not retry or automatically undo"
	}
	return context
}

func operationAction(old, new string) string {
	if filepath.Dir(old) != filepath.Dir(new) {
		return "move"
	}
	return "rename"
}
