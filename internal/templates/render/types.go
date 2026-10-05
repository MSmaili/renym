// Package render compiles and evaluates bounded, pure filename pipelines.
// It has no filesystem, environment, clock, provider, or mutation access.
package render

import (
	"fmt"
	"time"
)

const (
	MaxSourceBytes = 4096
	MaxParts       = 64
	MaxSteps       = 16
	MaxTokens      = 128
	MaxValueBytes  = 1024
	MaxNameBytes   = 255
)

// Error offsets refer to 1-based UTF-8 byte positions in the decoded filename
// string, not source-file columns (TOML/YAML may escape or span many lines).
type Error struct {
	Offset  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("filename byte %d: %s", e.Offset, e.Message) }

func at(offset int, message string) error { return &Error{Offset: offset + 1, Message: message} }

// Context values are collected by the application from original snapshots.
// Nil metadata means unavailable, not a zero-value default. Index must be > 0.
type Context struct {
	Name      string
	Directory bool
	Modified  *time.Time
	Size      *int64
	Index     int64
}

type Dependencies struct {
	Modified bool
	Size     bool
	Index    bool
}

type kind uint8

const (
	stringKind kind = iota
	integerKind
	timeKind
)

type field uint8

const (
	nameField field = iota
	stemField
	extField
	modifiedField
	sizeField
	indexField
)

type step struct {
	helper string
	width  int
	layout string
	offset int
}

type expression struct {
	field  field
	steps  []step
	offset int
}

type part struct {
	literal string
	expr    *expression
	offset  int
}

// Program is immutable and safe for concurrent reuse. Counters live in callers.
type Program struct {
	parts        []part
	dependencies Dependencies
}

func (p *Program) Dependencies() Dependencies { return p.dependencies }
