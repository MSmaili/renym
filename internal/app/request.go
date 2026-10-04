package app

import (
	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
)

// Request describes the existing one-shot mode workflow, independent of Cobra.
type Request struct {
	Path            string
	Mode            string
	Recursive       bool
	Directories     bool
	Files           bool
	Ignore          []string
	NoDefaultIgnore bool
	DryRun          bool
	SkipHistory     bool
	Command         string
	Version         string
}

// Plan keeps executable snapshots private; presentation receives a separate
// engine result. Later entry points must obtain plans from this service.
type Plan struct {
	Result     engine.PlanResult
	operations []fs.RenameOp
	root       string
	request    Request
}

type Result struct {
	Plan      engine.PlanResult
	Execution fs.Result
	HistoryID string
}
