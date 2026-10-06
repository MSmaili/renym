package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/templates"
)

func organizationFixture(t *testing.T) (string, string, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input, output := filepath.Join(base, "input"), filepath.Join(base, "output")
	if err := os.Mkdir(input, 0700); err != nil {
		t.Fatal(err)
	}
	return input, output, filepath.Join(base, "policy.toml")
}

func movePreset(root, directory string) string {
	result := fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", root)
	if directory != "" {
		result += fmt.Sprintf("directory=%q\n", directory)
	}
	return result
}

func TestOrganizationPreviewIsReadOnlyAndApplyIsBlocked(t *testing.T) {
	input, output, policy := organizationFixture(t)
	put(t, filepath.Join(input, "Screenshot One.PNG"), "image bytes")
	put(t, filepath.Join(input, "Notes.txt"), "document bytes")
	put(t, filepath.Join(input, "Other File.md"), "rename bytes")
	stamp := time.Date(2026, 10, 6, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(input, "Screenshot One.PNG"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	put(t, policy, fmt.Sprintf(`version=1
[[rules]]
id='images'
[rules.match]
extensions=['.png']
[rules.move]
root=%q
directory='photos/${file.modified | date("month")}'
[rules.rename]
filename='${file.stem | snake}${file.ext | lower}'
[[rules]]
id='documents'
[rules.match]
extensions=['.txt']
[rules.move]
root=%q
[[rules]]
id='rename'
[rules.rename]
mode='snake'
`, output, filepath.Join(filepath.Dir(output), "documents")))
	service := NewService(nil, forbiddenStore{})
	req := Request{Path: input, TemplatePath: policy, DryRun: true}
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 3 || !plan.PreviewOnly || plan.SourcePath != input || len(plan.operations) != 0 {
		t.Fatalf("organization preview: %+v %v", plan, err)
	}
	want := map[string]string{
		"Screenshot One.PNG": filepath.Join(output, "photos", "2026-10", "screenshot_one.png"),
		"Notes.txt":          filepath.Join(filepath.Dir(output), "documents", "Notes.txt"),
		"Other File.md":      filepath.Join(input, "other_file.md"),
	}
	for _, op := range plan.Result.Operations {
		if want[filepath.Base(op.OldPath)] != op.NewPath {
			t.Fatalf("wrong target: %+v", op)
		}
	}
	if len(plan.DirectoriesToCreate) != 4 {
		t.Fatalf("missing mkdir preview: %v", plan.DirectoriesToCreate)
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, directory := range plan.DirectoriesToCreate {
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preview made directory: %s %v", directory, err)
		}
	}
	req.DryRun = false
	for _, skipHistory := range []bool{false, true} {
		req.SkipHistory = skipHistory
		if _, err := service.Rename(context.Background(), req); !errors.Is(err, ErrOrganizationPreviewOnly) {
			t.Fatalf("move apply was enabled: %v", err)
		}
	}
	plan.PreviewOnly = false // Editing presentation must not enable mutation.
	plan.request.DryRun = false
	if _, err := service.Execute(context.Background(), plan); !errors.Is(err, ErrOrganizationPreviewOnly) {
		t.Fatalf("private preview gate bypassed: %v", err)
	}
	bytesAt(t, filepath.Join(input, "Screenshot One.PNG"), "image bytes")
	bytesAt(t, filepath.Join(input, "Notes.txt"), "document bytes")
	bytesAt(t, filepath.Join(input, "Other File.md"), "rename bytes")
}

func TestOrganizationFormatsPlanIdentically(t *testing.T) {
	input, output, policy := organizationFixture(t)
	put(t, filepath.Join(input, "Nested Dir", "Some File.txt"), "bytes")
	put(t, policy, strings.Replace(movePreset(output, "by-index/${index | pad(3)}"), "[[rules]]", "[selection]\nrecursive=true\n[[rules]]", 1))
	yaml := filepath.Join(filepath.Dir(policy), "policy.yaml")
	put(t, yaml, fmt.Sprintf("version: 1\nselection: {recursive: true}\nrules:\n - id: all\n   move: {root: %q, directory: 'by-index/${index | pad(3)}'}\n", output))
	service := NewService(nil, forbiddenStore{})
	a, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: yaml, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Result, b.Result) || !reflect.DeepEqual(a.Matches, b.Matches) || !reflect.DeepEqual(a.DirectoriesToCreate, b.DirectoriesToCreate) {
		t.Fatalf("format plans differ: %+v %+v", a, b)
	}
	if len(a.Result.Operations) != 1 || filepath.Base(a.Result.Operations[0].NewPath) != "Some File.txt" {
		t.Fatal("move-only renamed basename or included folders")
	}
}

func TestOrganizationConflictsAndNoFallback(t *testing.T) {
	input, output, policy := organizationFixture(t)
	put(t, filepath.Join(input, "A", "same.txt"), "first")
	put(t, filepath.Join(input, "B", "same.txt"), "second")
	put(t, filepath.Join(input, "C", "occupied.txt"), "source")
	put(t, filepath.Join(output, "occupied.txt"), "destination")
	put(t, policy, strings.Replace(movePreset(output, ""), "[[rules]]", "[selection]\nrecursive=true\n[[rules]]", 1)+"[[rules]]\nid='fallback'\n[rules.rename]\nmode='upper'\n")
	plan, err := NewService(nil, forbiddenStore{}).Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Result.Collisions) != 2 {
		t.Fatalf("conflicts: %+v %v", plan, err)
	}
	for _, match := range plan.Matches {
		if match.RuleID != "all" {
			t.Fatal("fallback after collision")
		}
	}
	bytesAt(t, filepath.Join(input, "A", "same.txt"), "first")
	bytesAt(t, filepath.Join(input, "B", "same.txt"), "second")
	bytesAt(t, filepath.Join(input, "C", "occupied.txt"), "source")
	bytesAt(t, filepath.Join(output, "occupied.txt"), "destination")
}

func TestOrganizationRejectsOverlapAndLinks(t *testing.T) {
	input, output, policy := organizationFixture(t)
	put(t, filepath.Join(input, "Some File.txt"), "bytes")
	service := NewService(nil, forbiddenStore{})
	for _, root := range []string{input, filepath.Join(input, "nested"), filepath.Dir(input)} {
		put(t, policy, movePreset(root, ""))
		if _, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true}); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Fatalf("overlap accepted: %v", err)
		}
	}
	put(t, policy, movePreset(output, ""))
	kind := "both"
	if _, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true, SelectionOverrides: SelectionOverrides{Kind: &kind}}); err == nil {
		t.Fatal("directory move override accepted")
	}
	put(t, filepath.Join(output, "link-target", "child", "existing"), "sentinel")
	link := filepath.Join(output, "escape")
	if err := os.Symlink(filepath.Join(output, "link-target"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	put(t, policy, movePreset(output, "escape/child"))
	plan, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 0 || len(plan.Result.Skipped) != 1 || !strings.Contains(plan.Result.Skipped[0].Reason, "link") {
		t.Fatalf("linked ancestor accepted: %+v %v", plan, err)
	}
}

func TestTemplateRuntimeInputRenameApplyAndUndo(t *testing.T) {
	input, _, policy := organizationFixture(t)
	put(t, filepath.Join(input, "Some File.txt"), "bytes")
	_, _, service, _ := setup(t)
	put(t, policy, "version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n")
	if _, err := service.Rename(context.Background(), Request{Path: input, TemplatePath: policy}); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(input, "some_file.txt"), "bytes")
	if _, err := service.Undo(context.Background(), input, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(input, "Some File.txt"), "bytes")
}

func TestOrganizationRequiresExplicitDirectoryInput(t *testing.T) {
	input, output, policy := organizationFixture(t)
	file := filepath.Join(input, "Some File.txt")
	put(t, file, "bytes")
	put(t, policy, movePreset(output, ""))
	service := NewService(nil, forbiddenStore{})
	for _, test := range []struct {
		input   string
		message string
	}{
		{"", "explicit input folder"},
		{file, "existing directory"},
		{filepath.Join(input, "missing"), ""},
	} {
		_, err := service.Plan(context.Background(), Request{Path: test.input, TemplatePath: policy, DryRun: true})
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatalf("invalid input %q: %v", test.input, err)
		}
	}
	// Existing rename-only templates still default to cwd; an explicitly
	// selected dot directory remains distinct from an omitted organization input.
	put(t, policy, "version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n")
	req, _, _, err := prepareRequest(Request{TemplatePath: policy, DryRun: true})
	if err != nil || req.Path != "." {
		t.Fatalf("rename cwd default changed: %+v %v", req, err)
	}
	put(t, policy, movePreset(output, ""))
	req, _, _, err = prepareRequest(Request{Path: ".", TemplatePath: policy, DryRun: true})
	if err != nil || req.Path != "." {
		t.Fatalf("explicit dot input rejected: %+v %v", req, err)
	}
	bytesAt(t, file, "bytes")
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid input created destination: %v", err)
	}
}

func TestOrganizationTemplateReusableAcrossInputsAndPlansFrozen(t *testing.T) {
	input, output, policy := organizationFixture(t)
	second := filepath.Join(filepath.Dir(input), "second-input")
	put(t, filepath.Join(input, "Same Name.txt"), "first")
	put(t, filepath.Join(second, "Same Name.txt"), "second")
	data := movePreset(output, "documents")
	put(t, policy, data)
	service := NewService(nil, forbiddenStore{})
	var plans []Plan
	for _, root := range []string{input, second} {
		plan, err := service.Plan(context.Background(), Request{Path: root, TemplatePath: policy, DryRun: true})
		if err != nil || len(plan.Result.Operations) != 1 {
			t.Fatalf("reused template: %+v %v", plan, err)
		}
		op := plan.Result.Operations[0]
		if op.OldPath != filepath.Join(root, "Same Name.txt") || op.NewPath != filepath.Join(output, "documents", "Same Name.txt") || plan.SourcePath != root {
			t.Fatalf("template captured input folder: %+v", plan)
		}
		plans = append(plans, plan)
	}
	contents, err := os.ReadFile(policy)
	if err != nil || string(contents) != data {
		t.Fatal("running reusable template modified its contents")
	}
	// A subsequent load sees edits; existing plans never reload the origin.
	replacement := filepath.Join(filepath.Dir(output), "replacement")
	put(t, policy, movePreset(replacement, ""))
	for _, plan := range plans {
		result, err := service.Execute(context.Background(), plan)
		if err != nil || len(result.Plan.Operations) != 1 || result.Plan.Operations[0].NewPath != filepath.Join(output, "documents", "Same Name.txt") {
			t.Fatalf("existing plan reloaded changed template: %+v %v", result, err)
		}
	}
	plan, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 1 || plan.Result.Operations[0].NewPath != filepath.Join(replacement, "Same Name.txt") {
		t.Fatalf("new plan missed template edit: %+v %v", plan, err)
	}
	bytesAt(t, filepath.Join(input, "Same Name.txt"), "first")
	bytesAt(t, filepath.Join(second, "Same Name.txt"), "second")
	for _, root := range []string{output, replacement} {
		if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preview created destination: %v", err)
		}
	}
}

func TestHomeAnchoredRootResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := resolveDeclaredRoot("~/Downloads")
	if err != nil || got != filepath.Join(home, "Downloads") {
		t.Fatalf("home root: %q %v", got, err)
	}
	for _, root := range []string{"./relative", "~user/Downloads", "~/../escape"} {
		if _, err := resolveDeclaredRoot(root); err == nil {
			t.Fatalf("root accepted: %q", root)
		}
	}
	if err := templates.ValidateRoot(filepath.Join(home, "Downloads")); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizationRejectsCrossFilesystemPreview(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux fixture uses the separate /dev/shm filesystem")
	}
	if info, err := os.Stat("/dev/shm"); err != nil || !info.IsDir() {
		t.Skip("no /dev/shm filesystem")
	}
	input, _, policy := organizationFixture(t)
	put(t, filepath.Join(input, "Some File.txt"), "bytes")
	id, err := fs.NewAdapter().PathIdentifier(input)
	if err != nil {
		t.Fatal(err)
	}
	other, err := fs.NewAdapter().PathIdentifier("/dev/shm")
	if err != nil {
		t.Skip(err)
	}
	if volumeIdentity(id) == volumeIdentity(other) {
		t.Skip("fixture directories are on the same filesystem")
	}
	put(t, policy, movePreset("/dev/shm", "renym-preview-fixture"))
	plan, err := NewService(nil, forbiddenStore{}).Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 0 || len(plan.Result.Skipped) != 1 || !strings.Contains(plan.Result.Skipped[0].Reason, "cross-filesystem") || len(plan.DirectoriesToCreate) != 0 {
		t.Fatalf("cross-filesystem preview: %+v %v", plan, err)
	}
	bytesAt(t, filepath.Join(input, "Some File.txt"), "bytes")
}

func TestOrganizationProtectsHistoryDestinations(t *testing.T) {
	input, _, policy := organizationFixture(t)
	_, store, service, _ := setup(t)
	put(t, filepath.Join(input, "Some File.txt"), "bytes")
	put(t, policy, movePreset(store.Directory(), "nested"))
	plan, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 0 || len(plan.Result.Skipped) != 1 || !strings.Contains(plan.Result.Skipped[0].Reason, "history") || len(plan.DirectoriesToCreate) != 0 {
		t.Fatalf("history destination: %+v %v", plan, err)
	}
}
