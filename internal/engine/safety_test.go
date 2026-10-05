package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/fs"
)

func TestPlanRejectsNamesBeforePathJoining(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", ".", "..", "../outside", "sub/name", `sub\name`, "bad\x00name", strings.Repeat("a", 256)} {
		e := NewEngine(mockMode{transform: func(string) string { return invalid }}, &mockAdapter{caseSensitive: true})
		plan := e.Plan([]string{source})
		if len(plan.Operations) != 0 || len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "invalid final name" {
			t.Fatalf("unsafe name %q planned: %+v", invalid, plan)
		}
	}
}

func TestRejectedDependencyDoesNotAuthorizeOverwrite(t *testing.T) {
	root := t.TempDir()
	a, b, c := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "c")
	for path, data := range map[string]string{a: "A", b: "B", c: "C"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	e := NewEngine(mockMode{transform: func(name string) string {
		if name == "a" {
			return "b"
		}
		return "c"
	}}, &mockAdapter{caseSensitive: true})
	plan := e.Plan([]string{a, b})
	if len(plan.Operations) != 0 {
		t.Fatalf("rejected dependency escaped: %+v", plan)
	}
	if err := fs.Apply(nil, false); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{a: "A", b: "B", c: "C"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("bytes lost at %s: %q, %v", path, data, err)
		}
	}
}

func TestCaseOnlyAliasHasAnExplicitDiagnostic(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "Mixed.txt"), filepath.Join(root, "mixed.txt")
	if err := os.WriteFile(source, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		t.Skip("test filesystem is case-sensitive")
	}
	e := NewEngine(mockMode{transform: strings.ToLower}, &mockAdapter{caseSensitive: false})
	plan := e.Plan([]string{source})
	if len(plan.Operations) != 0 || len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "case-only rename unsupported on this filesystem" {
		t.Fatalf("alias diagnostic missing: %+v", plan)
	}
}

func TestGeneratedNamesCannotBypassBasenameValidation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(nil, &mockAdapter{caseSensitive: true})
	plan := e.PlanNames([]string{source}, func(string) (string, string) { return "../escape", "" })
	if len(plan.Operations) != 0 || len(plan.Skipped) != 1 || plan.Skipped[0].Reason != "invalid final name" {
		t.Fatalf("generated basename escaped planner: %+v", plan)
	}
}
