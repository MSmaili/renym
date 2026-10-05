package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/templates"
)

const parityTOML = `version=1
name='Parity'
[selection]
kind='both'
recursive=true
ignore=['*.tmp']
[[rules]]
id='images'
[rules.match]
extensions=['.png']
[rules.rename]
filename='shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}'
[[rules]]
id='text'
[rules.match]
extensions=['.txt']
[rules.rename]
mode='snake'
[[rules]]
id='fallback'
[rules.rename]
filename='${file.stem | snake}${file.ext}'
`

const parityYAML = `version: 1
name: Parity
selection:
  kind: both
  recursive: true
  ignore: ['*.tmp']
rules:
  - id: images
    match: {extensions: ['.png']}
    rename:
      filename: 'shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}'
  - id: text
    match: {extensions: ['.txt']}
    rename: {mode: snake}
  - id: fallback
    rename:
      filename: '${file.stem | snake}${file.ext}'
`

func TestFormatsSharePlanApplyHistoryAndRecursiveUndo(t *testing.T) {
	root, store, service, req := setup(t)
	for _, name := range []string{"A Dir.v1/B Dir/Some File.txt", "Screenshot One.PNG", "My File.txt", "My-File.txt", "Other File.md", "Ignored File.tmp", ".env"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		put(t, path, name)
		stamp := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	req.Mode = ""
	tomlPath, yamlPath := filepath.Join(t.TempDir(), "parity.toml"), filepath.Join(t.TempDir(), "parity.yml")
	put(t, tomlPath, parityTOML)
	put(t, yamlPath, parityYAML)
	req.TemplatePath = tomlPath
	baseline, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	req.TemplatePath = yamlPath
	apply, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(baseline.Result, apply.Result) || !reflect.DeepEqual(baseline.Matches, apply.Matches) || !reflect.DeepEqual(baseline.Selection, apply.Selection) || !reflect.DeepEqual(baseline.templateSpec, apply.templateSpec) {
		t.Fatal("TOML/YAML changed normalized spec or plan")
	}
	if len(apply.Result.Collisions) != 1 || len(apply.Result.Operations) != 7 {
		t.Fatalf("fixture no longer exercises collisions/nesting: %+v", apply.Result)
	}
	req.DryRun = true
	preview, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(apply.Result, preview.Result) {
		t.Fatal("preview/apply plans disagree")
	}
	if _, err := service.Execute(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("preview wrote history: %v", err)
	}
	// Apply uses the frozen plan; undo never needs the original format/reference.
	put(t, yamlPath, "invalid: true")
	result, err := service.Execute(context.Background(), apply)
	if err != nil || len(result.Execution.Completed) != 7 {
		t.Fatalf("YAML apply: %+v %v", result, err)
	}
	bytesAt(t, filepath.Join(root, "a_dir_v_1", "b_dir", "some_file.txt"), "A Dir.v1/B Dir/Some File.txt")
	bytesAt(t, filepath.Join(root, "shot_2026-10-05_010203_001.png"), "Screenshot One.PNG")
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(entry.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "Parity") || !strings.Contains(string(encoded), "file.modified") || strings.Contains(string(encoded), "invalid") {
		t.Fatalf("history reloaded YAML: %s", encoded)
	}
	if _, err := service.Undo(context.Background(), root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A Dir.v1/B Dir/Some File.txt", "Screenshot One.PNG", "My File.txt", "My-File.txt", "Other File.md", "Ignored File.tmp", ".env"} {
		bytesAt(t, filepath.Join(root, filepath.FromSlash(name)), name)
	}
}

func TestNamedTemplateProtectsResolvedFileAndFreezesOrigin(t *testing.T) {
	root, store, service, req := setup(t)
	base := filepath.Join(root, "Config Folder")
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("APPDATA", base)
	dir, err := templates.Directory()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "My-Preset.yaml")
	put(t, path, "version: 1\nname: Frozen\nselection: {kind: both, recursive: true}\nrules:\n  - id: all\n    rename: {mode: snake}\n")
	put(t, filepath.Join(root, "Some File.txt"), "unchanged bytes")
	req.Mode, req.TemplatePath = "", "My-Preset"
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TemplatePath != path || plan.request.TemplatePath != path || len(plan.Result.Operations) != 1 {
		t.Fatalf("name was not resolved/protected: %+v", plan)
	}
	if len(plan.Result.Skipped) < 2 {
		t.Fatal("resolved file and ancestors not protected")
	}
	// A newly ambiguous name cannot alter an already frozen executable plan.
	put(t, filepath.Join(dir, "My-Preset.toml"), screenshotPreset)
	put(t, path, "invalid: true")
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(entry.Config)
	if err != nil {
		t.Fatal(err)
	}
	var config Request
	if err := json.Unmarshal(encoded, &config); err != nil {
		t.Fatal(err)
	}
	if config.TemplatePath != path {
		t.Fatalf("history did not freeze origin: %s", encoded)
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "Some File.txt"), "unchanged bytes")
	bytesAt(t, path, "invalid: true")
}

func TestYAMLAndNamedErrorsPrecedeInputDiscovery(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("APPDATA", base)
	dir, err := templates.Directory()
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "invalid.yaml"), "version: 1\nrules:\n - id: all\n   rename: {mode: snake, filename: '${file.name}'}\n")
	put(t, filepath.Join(dir, "ambiguous.yaml"), parityYAML)
	put(t, filepath.Join(dir, "ambiguous.toml"), parityTOML)
	service := NewService(nil, forbiddenStore{})
	for _, ref := range []string{"invalid", "ambiguous", "missing"} {
		_, err := service.Rename(context.Background(), Request{Path: filepath.Join(base, "no-input"), TemplatePath: ref})
		if err == nil || !strings.Contains(err.Error(), ref) || strings.Contains(err.Error(), "no-input") {
			t.Fatalf("template error did not precede discovery: %q %v", ref, err)
		}
	}
}
