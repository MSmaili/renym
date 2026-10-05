// Package templates loads declarative rename presets. It does not discover or
// mutate input files, run arbitrary code, or perform classification.
package templates

const (
	Version         = 1
	MaxBytes        = 64 * 1024
	MaxRules        = 128
	MaxPatterns     = 32
	MaxPatternBytes = 1024
)

// Spec is a normalized, format-neutral preset snapshot for previews/history.
type Spec struct {
	Version   int       `json:"version"`
	Name      string    `json:"name,omitempty"`
	Selection Selection `json:"selection"`
	Rules     []Rule    `json:"rules"`
}

type Selection struct {
	Kind            string   `json:"kind"`
	Recursive       bool     `json:"recursive"`
	Ignore          []string `json:"ignore,omitempty"`
	NoDefaultIgnore bool     `json:"no_default_ignore"`
}

type Rule struct {
	ID     string `toml:"id" json:"id"`
	Match  Match  `toml:"match" json:"match"`
	Rename Rename `toml:"rename" json:"rename"`
}

type Match struct {
	Glob       []string `toml:"glob" json:"glob,omitempty"`
	Extensions []string `toml:"extensions" json:"extensions,omitempty"`
}

type Rename struct {
	Mode     string  `toml:"mode" json:"mode,omitempty"`
	Filename *string `toml:"filename" json:"filename,omitempty"`
}

// Pointer fields in the wire representation distinguish omission from explicit
// empty values. Future adapters normalize into the same immutable Spec.
type document struct {
	Version   int     `toml:"version"`
	Name      *string `toml:"name"`
	Selection struct {
		Kind            *string  `toml:"kind"`
		Recursive       bool     `toml:"recursive"`
		Ignore          []string `toml:"ignore"`
		NoDefaultIgnore bool     `toml:"no_default_ignore"`
	} `toml:"selection"`
	Rules []Rule `toml:"rules"`
}
