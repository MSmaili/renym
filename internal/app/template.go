package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/templates"
)

// prepareRequest validates the selected configuration before input discovery.
// It is shared by every entry point, not a Cobra-specific preset expansion.
func prepareRequest(req Request) (Request, *templates.Compiled, []string, error) {
	if req.TemplatePath == "" {
		if _, ok := engine.ModeRegistry[req.Mode]; !ok {
			return req, nil, nil, fmt.Errorf("unknown rename mode %q", req.Mode)
		}
		return req, nil, nil, nil
	}
	if req.Mode != "" {
		return req, nil, nil, fmt.Errorf("mode and template are mutually exclusive")
	}
	req.SelectionOverrides = cloneOverrides(req.SelectionOverrides)
	compiled, err := templates.Load(req.TemplatePath)
	if err != nil {
		return req, nil, nil, err
	}
	req.TemplatePath = compiled.SourcePath()
	if req.Path == "" {
		if compiled.HasMoves() {
			return req, nil, nil, fmt.Errorf("organization requires an explicit input folder; provide --path")
		}
		req.Path = "."
	}
	if compiled.HasMoves() && !req.DryRun && req.SkipHistory {
		return req, nil, nil, ErrOrganizationHistoryRequired
	}
	selection := compiled.Selection()
	var overrides []string
	if req.SelectionOverrides.Kind != nil {
		selection.Kind = *req.SelectionOverrides.Kind
		overrides = append(overrides, "kind")
	}
	if req.SelectionOverrides.Recursive != nil {
		selection.Recursive = *req.SelectionOverrides.Recursive
		overrides = append(overrides, "recursive")
	}
	if req.SelectionOverrides.Ignore != nil {
		selection.Ignore = slices.Clone(*req.SelectionOverrides.Ignore)
		overrides = append(overrides, "ignore")
	}
	if req.SelectionOverrides.NoDefaultIgnore != nil {
		selection.NoDefaultIgnore = *req.SelectionOverrides.NoDefaultIgnore
		overrides = append(overrides, "no_default_ignore")
	}
	if !slices.Contains([]string{"files", "directories", "both"}, selection.Kind) {
		return req, nil, nil, fmt.Errorf("selection override kind: expected files, directories, or both")
	}
	if compiled.HasMoves() && selection.Kind != "files" {
		return req, nil, nil, fmt.Errorf("move templates support only regular-file selection")
	}
	req.Files, req.Directories = selection.Kind != "directories", selection.Kind != "files"
	req.Recursive, req.Ignore, req.NoDefaultIgnore = selection.Recursive, selection.Ignore, selection.NoDefaultIgnore
	return req, compiled, overrides, nil
}

func cloneValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneOverrides(overrides SelectionOverrides) SelectionOverrides {
	copy := SelectionOverrides{
		Kind: cloneValue(overrides.Kind), Recursive: cloneValue(overrides.Recursive),
		NoDefaultIgnore: cloneValue(overrides.NoDefaultIgnore),
	}
	if overrides.Ignore != nil {
		values := slices.Clone(*overrides.Ignore)
		copy.Ignore = &values
	}
	return copy
}

// Protect the explicitly selected policy file and its ancestors from a run
// renaming its own configuration, including ordinary existing path aliases.
func protectsTemplate(candidate, filename string) bool {
	if filename == "" {
		return false
	}
	contains := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if contains(candidate, filename) {
		return true
	}
	actualFile, fileErr := filepath.EvalSymlinks(filename)
	actualPath, pathErr := filepath.EvalSymlinks(candidate)
	return fileErr == nil && pathErr == nil && contains(actualPath, actualFile)
}
