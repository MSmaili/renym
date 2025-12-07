package history

import "time"

type Entry struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`

	Path    string `json:"path"`
	Command string `json:"command"`

	Config     any         `json:"config"`
	Operations []Operation `json:"operations"`
	Skipped    []Skipped   `json:"skipped"`
	Collisions []Collision `json:"collisions"`
}

type Operation struct {
	Old string `json:"old"`
	New string `json:"new"`
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
