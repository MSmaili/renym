package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/templates"
)

const screenshotPreset = `version = 1
name = "Screenshots"
[[rules]]
id = "screenshots"
[rules.match]
glob = ["Screenshot*", "Screen Shot*"]
extensions = [".png"]
[rules.rename]
mode = "snake"
`

func withPreset(t *testing.T, req Request, source string) Request {
	t.Helper()
	req.Mode = ""
	req.TemplatePath = filepath.Join(t.TempDir(), "preset.toml")
	put(t, req.TemplatePath, source)
	return req
}

func TestTemplatePreviewApplyAndUndo(t *testing.T) {
	root, store, service, req := setup(t)
	old := filepath.Join(root, "Screenshot One.PNG")
	put(t, old, "screenshot bytes")
	put(t, filepath.Join(root, "Other File.txt"), "unmatched bytes")
	req = withPreset(t, req, screenshotPreset)
	req.DryRun = true
	preview, err := service.Plan(context.Background(), req)
	if err != nil || len(preview.Result.Operations) != 1 || len(preview.Matches) != 1 || len(preview.Result.Skipped) != 1 {
		t.Fatalf("wrong template plan: %+v, %v", preview, err)
	}
	if preview.Result.Operations[0].NewPath != filepath.Join(root, "screenshot_one.PNG") || preview.Matches[0].RuleID != "screenshots" {
		t.Fatalf("extension/rule changed: %+v", preview)
	}
	if _, err := service.Execute(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("preview created history: %v", err)
	}
	bytesAt(t, old, "screenshot bytes")
	req.DryRun = false
	result, err := service.Rename(context.Background(), req)
	if err != nil || len(result.Execution.Completed) != 1 {
		t.Fatalf("template apply: %+v, %v", result, err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	config := entry.Config.(map[string]any)
	if config["template_spec"].(map[string]any)["name"] != "Screenshots" {
		t.Fatalf("history lacks preset snapshot: %+v", config)
	}
	if _, err := service.Undo(context.Background(), root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, old, "screenshot bytes")
	bytesAt(t, filepath.Join(root, "Other File.txt"), "unmatched bytes")
}

func TestInvalidTemplatePrecedesDiscoveryAndNeverAccessesHistory(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "Hello World.txt")
	put(t, old, "original")
	service := NewService(nil, forbiddenStore{})
	req := withPreset(t, Request{Path: root}, strings.Replace(screenshotPreset, "version = 1", "version = 2", 1))
	for _, path := range []string{root, filepath.Join(root, "missing")} {
		req.Path = path
		if _, err := service.Rename(context.Background(), req); err == nil || !strings.Contains(err.Error(), "version") || !strings.Contains(err.Error(), strconv.Quote(req.TemplatePath)) {
			t.Fatalf("configuration was not rejected first: %v", err)
		}
	}
	bytesAt(t, old, "original")
	req.Mode = "snake"
	if _, err := service.Plan(context.Background(), req); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("mode/template exclusivity not enforced: %v", err)
	}
}

func TestTemplateFirstMatchWinsEvenForNoChange(t *testing.T) {
	root, _, service, req := setup(t)
	old := filepath.Join(root, "already-normal.txt")
	put(t, old, "keep")
	req = withPreset(t, req, `version=1
[[rules]]
id='first'
[rules.rename]
mode='kebab'
[[rules]]
id='later'
[rules.rename]
mode='snake'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 0 || len(plan.Matches) != 1 || plan.Matches[0].RuleID != "first" || len(plan.Result.Skipped) != 1 || plan.Result.Skipped[0].Reason != "no change" {
		t.Fatalf("rules chained after no-op: %+v, %v", plan, err)
	}
	bytesAt(t, old, "keep")
}

func TestTemplateCollisionsAcrossRulesPreserveBothFiles(t *testing.T) {
	root, _, service, req := setup(t)
	a, b := filepath.Join(root, "My File.png"), filepath.Join(root, "My-File.png")
	put(t, a, "A")
	put(t, b, "B")
	req = withPreset(t, req, `version=1
[[rules]]
id='spaces'
[rules.match]
glob=['* *']
[rules.rename]
mode='snake'
[[rules]]
id='fallback'
[rules.rename]
mode='snake'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Result.Collisions) != 1 || len(plan.Matches) != 2 || plan.Result.Operations[0].OldPath != a {
		t.Fatalf("cross-rule collision missed or unstable: %+v, %v", plan, err)
	}
	result, err := service.Execute(context.Background(), plan)
	if err != nil || len(result.Execution.Completed) != 1 {
		t.Fatalf("collision apply: %+v, %v", result, err)
	}
	bytesAt(t, filepath.Join(root, "my_file.png"), "A")
	bytesAt(t, b, "B")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, a, "A")
	bytesAt(t, b, "B")
}

func TestTemplateSelectionAndExplicitOverrides(t *testing.T) {
	root, _, service, req := setup(t)
	put(t, filepath.Join(root, "Parent Dir", "Child File.txt"), "nested")
	put(t, filepath.Join(root, "Top File.txt"), "top")
	put(t, filepath.Join(root, "Skipped File.tmp"), "ignored")
	put(t, filepath.Join(root, ".git", "Hidden File.txt"), "default ignored")
	req = withPreset(t, req, `version=1
[selection]
kind='both'
recursive=true
ignore=['*.tmp']
[[rules]]
id='all'
[rules.rename]
mode='snake'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 3 || len(plan.Overrides) != 0 {
		t.Fatalf("template defaults replaced by zero-valued request: %+v, %v", plan, err)
	}
	kind, recursive, ignore, noDefault := "files", false, []string{"Top*"}, true
	req.SelectionOverrides = SelectionOverrides{Kind: &kind, Recursive: &recursive, Ignore: &ignore, NoDefaultIgnore: &noDefault}
	plan, err = service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Overrides) != 4 || plan.Result.Operations[0].OldPath != filepath.Join(root, "Skipped File.tmp") {
		t.Fatalf("explicit overrides ignored or merged: %+v, %v", plan, err)
	}
	kind = "directories"
	recursive = true
	noDefault = false
	plan, err = service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || !plan.operations[0].Source.Mode.IsDir() {
		t.Fatalf("directory-only override failed: %+v, %v", plan, err)
	}
}

func TestTemplateRecursiveDirectoriesAndRootExclusion(t *testing.T) {
	root, _, service, req := setup(t)
	original := filepath.Join(root, "Parent Dir.v1", "Child Dir", "Some File.txt")
	put(t, original, "nested")
	req = withPreset(t, req, `version=1
[selection]
kind='both'
recursive=true
[[rules]]
id='all'
[rules.rename]
mode='snake'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 3 || len(plan.Matches) != 3 {
		t.Fatalf("nested plan: %+v, %v", plan, err)
	}
	for _, op := range plan.Result.Operations {
		if op.OldPath == root {
			t.Fatal("traversal root included")
		}
	}
	if plan.Result.Operations[0].OldPath != original {
		t.Fatal("physical child-first execution lost")
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "parent_dir_v_1", "child_dir", "some_file.txt"), "nested")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, original, "nested")
	req.Path = filepath.Dir(original)
	kind, recursive := "directories", false
	req.SelectionOverrides = SelectionOverrides{Kind: &kind, Recursive: &recursive}
	plan, err = service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 0 {
		t.Fatalf("single directory unexpectedly renames itself/files: %+v, %v", plan, err)
	}
}

func TestTemplatePlanFreezesConfigAndProtectsItsFile(t *testing.T) {
	root, store, service, req := setup(t)
	put(t, filepath.Join(root, "Some File.txt"), "keep")
	req = withPreset(t, req, "version=1\nname='Frozen'\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n")
	// The policy sits inside the selected input tree and matches its own rule.
	inside := filepath.Join(root, "Config Folder", "My Preset.toml")
	data, err := os.ReadFile(req.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	put(t, inside, string(data))
	req.TemplatePath = inside
	kind, recursive := "both", true
	req.SelectionOverrides = SelectionOverrides{Kind: &kind, Recursive: &recursive}
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Result.Skipped) != 2 {
		t.Fatalf("active policy renamed: %+v, %v", plan, err)
	}
	put(t, inside, "invalid changed config")
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(entry.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(serialized), "Frozen") || strings.Contains(string(serialized), "invalid changed config") {
		t.Fatalf("plan reloaded changed rules: %s", serialized)
	}
	bytesAt(t, inside, "invalid changed config")
}

func TestTemplatesRecognizeEveryExistingMode(t *testing.T) {
	for mode := range engine.ModeRegistry {
		source := strings.Replace(screenshotPreset, `mode = "snake"`, `mode = "`+mode+`"`, 1)
		if _, err := templates.Parse([]byte(source)); err != nil {
			t.Errorf("existing mode %s not supported: %v", mode, err)
		}
	}
}

func TestTemplateUsesSourceSnapshotsForStaleRevalidation(t *testing.T) {
	root, _, service, req := setup(t)
	old := filepath.Join(root, "Screenshot One.png")
	put(t, old, "original")
	req = withPreset(t, req, screenshotPreset)
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	put(t, old, "changed content length")
	if _, err := service.Execute(context.Background(), plan); !errors.Is(err, fs.ErrStalePlan) {
		t.Fatalf("template bypassed stale revalidation: %v", err)
	}
	bytesAt(t, old, "changed content length")
}

func TestTemplateSingleFileRespectsSelectionAndIgnores(t *testing.T) {
	root, _, service, req := setup(t)
	source := filepath.Join(root, "Screenshot One.png")
	put(t, source, "keep")
	req = withPreset(t, req, screenshotPreset)
	req.Path = source
	ignored := []string{"Screenshot*"}
	req.SelectionOverrides.Ignore = &ignored
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 0 {
		t.Fatalf("single-file ignore bypassed: %+v, %v", plan, err)
	}
	req.SelectionOverrides.Ignore = nil
	kind := "directories"
	req.SelectionOverrides.Kind = &kind
	plan, err = service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 0 {
		t.Fatalf("single-file directory selection bypassed: %+v, %v", plan, err)
	}
	kind = "files"
	plan, err = service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 {
		t.Fatalf("single file not planned: %+v, %v", plan, err)
	}
	bytesAt(t, source, "keep")
}

func TestTemplatePlanDoesNotRetainMutableOverridePointers(t *testing.T) {
	root, _, service, req := setup(t)
	put(t, filepath.Join(root, "Screenshot One.png"), "keep")
	req = withPreset(t, req, screenshotPreset)
	kind, recursive, ignore := "files", false, []string{"*.tmp"}
	req.SelectionOverrides = SelectionOverrides{Kind: &kind, Recursive: &recursive, Ignore: &ignore}
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	kind, recursive, ignore[0] = "directories", true, "*"
	if *plan.request.SelectionOverrides.Kind != "files" || *plan.request.SelectionOverrides.Recursive || (*plan.request.SelectionOverrides.Ignore)[0] != "*.tmp" || plan.Selection.Ignore[0] != "*.tmp" {
		t.Fatal("caller mutated a frozen plan's effective overrides")
	}
}
