package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/log"
)

func TestOrganizationFailureOutputDistinguishesNativeAndVerifiedResults(t *testing.T) {
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetErrorOutput(&output)
	t.Cleanup(func() { log.SetOutput(os.Stdout); log.SetErrorOutput(os.Stderr) })
	result := app.Result{Organization: true, HistoryID: "run.json", SourcePath: "input", RequiresReconciliation: true,
		Execution:         fs.Result{Completed: []fs.RenameOp{{OldPath: "old", NewPath: "new"}}},
		MoveOutcomes:      []fs.MoveOutcome{{Attempted: true, Completed: true}},
		DirectoryOutcomes: []fs.DirectoryOutcome{{Attempted: true, Completed: true}},
	}
	printOrganizationOutcome(result, true)
	cliRequireText(t, output.String(), "Run: run.json", "Unverified file mutation: native_completed=true", "Unverified directory mutation: native_completed=true", "Reconciliation required", "do not retry")
	if strings.Contains(output.String(), "Created directory:") || strings.Contains(output.String(), "Removed owned directory:") {
		t.Fatal("unverified directory was advertised as verified")
	}
	cause := errors.New("checkpoint failure")
	err := applicationFailure("undo", result, cause)
	if !errors.Is(err, cause) {
		t.Fatal("lost failure cause")
	}
	cliRequireText(t, err.Error(), "1 native file completion(s)", "0 verified directory creation(s)", "0 verified directory removal(s)")
	cliRequireText(t, err.Error(), "run run.json", "reconciliation required", "do not retry")
	output.Reset()
	result.RequiresReconciliation = false
	printUnverifiedOutcomes(result)
	if output.Len() != 0 {
		t.Fatal("known non-empty retention was labeled uncertain")
	}
}
