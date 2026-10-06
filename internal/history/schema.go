package history

import (
	"time"

	"github.com/MSmaili/renym/internal/fs"
)

const SchemaVersion = 1
const OrganizationSchemaVersion = 2

func VerifiedVersion(version int) bool {
	return version == SchemaVersion || version == OrganizationSchemaVersion
}

const (
	Pending            = "pending"
	Complete           = "complete"
	Partial            = "partial"
	Undoing            = "undoing"
	PartialUndo        = "partial_undo"
	OrganizationUndone = "organization_undone"
)

type Entry struct {
	SchemaVersion int         `json:"schema_version,omitempty"`
	ID            string      `json:"id,omitempty"`
	State         string      `json:"state,omitempty"`
	Intent        []Operation `json:"intent,omitempty"`
	Undone        int         `json:"undone,omitempty"`
	UndoFailed    *Failure    `json:"undo_failed,omitempty"`
	Failed        *Failure    `json:"failed,omitempty"`
	Unattempted   []Operation `json:"unattempted,omitempty"`
	Version       string      `json:"version"`
	Timestamp     time.Time   `json:"timestamp"`

	Path  string `json:"path"`
	DirID string `json:"dir_id"`

	Command string `json:"command"`

	Config       any           `json:"config"`
	Operations   []Operation   `json:"operations"`
	Skipped      []Skipped     `json:"skipped"`
	Collisions   []Collision   `json:"collisions"`
	Organization *Organization `json:"organization,omitempty"`
}

type Operation struct {
	Old    string          `json:"old"`
	New    string          `json:"new"`
	ID     int             `json:"id,omitempty"`
	Source *fs.Snapshot    `json:"source,omitempty"`
	Move   *fs.MoveRequest `json:"move,omitempty"`
}

type Organization struct {
	SourceRoot       string                `json:"source_root"`
	SourceSnapshot   *fs.Snapshot          `json:"source_snapshot"`
	DirectoryIntents []fs.DirectoryRequest `json:"directory_intents,omitempty"`
	Directories      []fs.OwnedDirectory   `json:"directories,omitempty"`
	Cleaned          int                   `json:"cleaned,omitempty"`
	Retained         []DirectoryRetention  `json:"retained,omitempty"`
	Active           *OrganizationStep     `json:"active,omitempty"`
	Bindings         []DirectoryBinding    `json:"bindings"`
	Error            string                `json:"error,omitempty"`
}

type DirectoryBinding struct {
	Request  fs.DirectoryRequest `json:"request"`
	Snapshot *fs.Snapshot        `json:"snapshot"`
}

type OrganizationStep struct {
	Action           string               `json:"action"`
	OperationID      int                  `json:"operation_id,omitempty"`
	Directory        *fs.DirectoryRequest `json:"directory,omitempty"`
	MoveOutcome      *fs.MoveOutcome      `json:"move_outcome,omitempty"`
	Move             *fs.MoveRequest      `json:"move,omitempty"`
	DirectoryOutcome *fs.DirectoryOutcome `json:"directory_outcome,omitempty"`
	Error            string               `json:"error,omitempty"`
}

type DirectoryRetention struct {
	Directory fs.OwnedDirectory `json:"directory"`
	Reason    string            `json:"reason"`
}

type Failure struct {
	Operation Operation `json:"operation"`
	Error     string    `json:"error"`
}

type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type Collision struct {
	Source1 string `json:"source1"`
	Source2 string `json:"source2"`
	Target  string `json:"target"`
}
