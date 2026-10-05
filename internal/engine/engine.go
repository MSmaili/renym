package engine

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MSmaili/renym/internal/fs"
)

type PlanResult struct {
	Operations []RenameOp
	Skipped    []SkippedFile
	Collisions []Collision
}

type SkippedFile struct {
	Path   string
	Reason string
}

type Collision struct {
	Source1 string
	Source2 string
	Target  string
}

type RenameOp struct {
	OldPath string
	NewPath string
}
type FileSystemAdapter interface {
	IsCaseSensitive() bool
	SanitizeName(name string) string
}

type Engine struct {
	adapter FileSystemAdapter
	mode    RenameMode
}

func NewEngine(mode RenameMode, adapter FileSystemAdapter) *Engine {
	return &Engine{
		mode:    mode,
		adapter: adapter,
	}
}

func (e *Engine) Plan(paths []string) PlanResult {
	return e.PlanNames(paths, func(path string) (string, string) {
		return e.computeNewName(path), ""
	})
}

// PlanNames applies the existing validation/conflict policy to generated full
// basenames. A reason skips an item; the generator cannot choose a directory or
// bypass final-name validation. Input order is the conflict winner order.
func (e *Engine) PlanNames(paths []string, generate func(string) (name, reason string)) PlanResult {
	planResult := PlanResult{
		Operations: []RenameOp{},
		Skipped:    []SkippedFile{},
		Collisions: []Collision{},
	}

	caseSensitive := e.adapter.IsCaseSensitive()

	type pendingOp struct {
		oldPath        string
		newPath        string
		newPathCompare string
	}

	var pending []pendingOp

	for _, path := range paths {
		if err := fs.ValidateName(filepath.Base(path)); err != nil {
			e.addSkipped(&planResult, path, "unsupported source name for reversible rename")
			continue
		}
		newName, reason := generate(path)
		if reason != "" {
			e.addSkipped(&planResult, path, reason)
			continue
		}
		if err := fs.ValidateName(newName); err != nil {
			e.addSkipped(&planResult, path, "invalid final name")
			continue
		}
		newPath := filepath.Join(filepath.Dir(path), newName)
		newPathCompare := compareKey(newPath, caseSensitive)

		if newPath == path {
			e.addSkipped(&planResult, path, "no change")
			continue
		}

		pending = append(pending, pendingOp{
			oldPath:        path,
			newPath:        newPath,
			newPathCompare: newPathCompare,
		})

	}

	seen := make(map[string]string, len(pending))

	for _, op := range pending {
		if e.hasDiskCollision(op.newPath) {
			reason := "target already exists"
			if strings.EqualFold(op.oldPath, op.newPath) {
				oldInfo, oldErr := os.Lstat(op.oldPath)
				newInfo, newErr := os.Lstat(op.newPath)
				if oldErr == nil && newErr == nil && os.SameFile(oldInfo, newInfo) {
					reason = "case-only rename unsupported on this filesystem"
				}
			}
			e.addSkipped(&planResult, op.oldPath, reason)
			e.addCollision(&planResult, op.newPath, op.oldPath, op.newPath)
			continue
		}

		if existingSource, exists := seen[op.newPathCompare]; exists {
			e.addSkipped(&planResult, op.oldPath, "duplicate target in batch")
			e.addCollision(&planResult, existingSource, op.oldPath, op.newPath)
			continue
		}

		planResult.Operations = append(planResult.Operations, RenameOp{
			OldPath: op.oldPath,
			NewPath: op.newPath,
		})

		seen[op.newPathCompare] = op.oldPath
	}

	return planResult
}

func (e *Engine) computeNewPathPerSelectedMode(path string) string {
	return filepath.Join(filepath.Dir(path), e.computeNewName(path))
}

func (e *Engine) computeNewName(path string) string {
	info, err := os.Lstat(path)
	return ModeBasename(filepath.Base(path), err == nil && info.IsDir(), e.mode, e.adapter)
}

// ModeBasename preserves the existing mode/sanitization/extension behavior
// without reading the filesystem. Directory kinds come from original snapshots.
func ModeBasename(name string, directory bool, mode RenameMode, adapter FileSystemAdapter) string {
	ext := filepath.Ext(name)
	if directory {
		ext = ""
	}
	stem := strings.TrimSuffix(name, ext)
	return mode.Transform(adapter.SanitizeName(stem)) + ext
}

// compareKey returns the comparison key for a path based on case sensitivity
func compareKey(path string, caseSensitive bool) string {
	if !caseSensitive {
		return strings.ToLower(path)
	}
	return path
}

// hasDiskCollision deliberately rejects every occupied destination, including
// other batch sources and dangling symlinks. Sequential execution cannot safely
// perform swaps/chains. Inaccessible destinations are also rejected, not guessed.
func (e *Engine) hasDiskCollision(newPath string) bool {
	_, err := os.Lstat(newPath)
	return !errors.Is(err, os.ErrNotExist)
}

// addSkipped adds a file to the skipped list
func (e *Engine) addSkipped(result *PlanResult, path, reason string) {
	result.Skipped = append(result.Skipped, SkippedFile{
		Path:   path,
		Reason: reason,
	})
}

// addCollision adds a collision to the collision list
func (e *Engine) addCollision(result *PlanResult, source1, source2, target string) {
	result.Collisions = append(result.Collisions, Collision{
		Source1: source1,
		Source2: source2,
		Target:  target,
	})
}

// pathDepth returns the depth of a path by counting separators
func pathDepth(path string) int {
	return strings.Count(path, string(filepath.Separator))
}

// SortPathsByDepth sorts paths with deepest paths first to ensure
// safe recursive directory renaming (children before parents)
func (e *Engine) SortPathsByDepth(paths []string) []string {
	sorted := make([]string, len(paths))
	copy(sorted, paths)

	sort.SliceStable(sorted, func(i, j int) bool {
		return pathDepth(sorted[i]) > pathDepth(sorted[j])
	})

	return sorted
}
