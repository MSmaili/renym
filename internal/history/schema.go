package history

import (
	"time"

	"github.com/MSmaili/renym/internal/fs"
)

const SchemaVersion = 1

const (
	Pending     = "pending"
	Complete    = "complete"
	Partial     = "partial"
	Undoing     = "undoing"
	PartialUndo = "partial_undo"
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

	Config     any         `json:"config"`
	Operations []Operation `json:"operations"`
	Skipped    []Skipped   `json:"skipped"`
	Collisions []Collision `json:"collisions"`
}

type Operation struct {
	Old    string       `json:"old"`
	New    string       `json:"new"`
	ID     int          `json:"id,omitempty"`
	Source *fs.Snapshot `json:"source,omitempty"`
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
