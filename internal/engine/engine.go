package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	beingRenamed := make(map[string]bool, len(paths))

	for _, path := range paths {
		newPath := e.computeNewPathPerSelectedMode(path)
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

		beingRenamed[compareKey(path, caseSensitive)] = true
	}

	seen := make(map[string]string, len(pending))

	for _, op := range pending {
		if e.hasDiskCollision(op.newPath, op.newPathCompare, beingRenamed) {
			e.addSkipped(&planResult, op.oldPath, "target already exists")
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
	dir := filepath.Dir(path)
	oldName := filepath.Base(path)

	ext := filepath.Ext(oldName)
	nameWithoutExt := strings.TrimSuffix(oldName, ext)

	transformedName := e.mode.Transform(nameWithoutExt)
	transformedName = e.adapter.SanitizeName(transformedName)
	newName := transformedName + ext

	return filepath.Join(dir, newName)
}

// compareKey returns the comparison key for a path based on case sensitivity
func compareKey(path string, caseSensitive bool) string {
	if !caseSensitive {
		return strings.ToLower(path)
	}
	return path
}

// hasDiskCollision checks if the target path exists on disk and is not being renamed away
func (e *Engine) hasDiskCollision(newPath, compareKey string, beingRenamed map[string]bool) bool {
	if _, err := os.Stat(newPath); err == nil {
		return !beingRenamed[compareKey]
	}
	return false
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
