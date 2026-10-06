package app

import (
	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/templates"
)

// Request describes a one-shot mode or template workflow, independent of Cobra.
type Request struct {
	Path               string // explicit input; empty means cwd for rename, invalid for organization
	Mode               string
	TemplatePath       string // explicit path or configured name; plans freeze the resolved path
	SelectionOverrides SelectionOverrides
	Recursive          bool
	Directories        bool
	Files              bool
	Ignore             []string
	NoDefaultIgnore    bool
	DryRun             bool
	SkipHistory        bool
	Command            string
	Version            string
}

// Presence, not zero values, determines whether an adapter overrides a preset.
// --directories=false means files; --dirs-only=false restores ordinary selection.
type SelectionOverrides struct {
	Kind            *string   `json:"kind,omitempty"`
	Recursive       *bool     `json:"recursive,omitempty"`
	Ignore          *[]string `json:"ignore,omitempty"`
	NoDefaultIgnore *bool     `json:"no_default_ignore,omitempty"`
}

type RuleMatch struct {
	Path     string `json:"path"`
	RuleID   string `json:"rule_id"`
	Mode     string `json:"mode"`
	Filename bool   `json:"filename,omitempty"`
	Move     bool   `json:"move,omitempty"`
	Index    int64  `json:"index"`
}

// Plan keeps executable snapshots private; presentation receives a separate
// engine result. Later entry points must obtain plans from this service.
type Plan struct {
	Result              engine.PlanResult
	TemplatePath        string
	TemplateName        string
	Selection           templates.Selection
	Overrides           []string
	Matches             []RuleMatch
	SourcePath          string
	PreviewOnly         bool
	DirectoriesToCreate []string
	previewOnly         bool
	operations          []fs.RenameOp
	root                string
	request             Request
	templateSpec        *templates.Spec
}

type Result struct {
	Plan      engine.PlanResult
	Execution fs.Result
	HistoryID string
}
