package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/fs"
)

func TestFileTargetsRejectNamespaceAndDuplicateConflicts(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "A.txt"), filepath.Join(root, "B.txt")
	for _, file := range []string{a, b} {
		if err := os.WriteFile(file, []byte(file), 0600); err != nil {
			t.Fatal(err)
		}
	}
	e := NewEngine(nil, fs.NewAdapter())
	for _, reverse := range []bool{false, true} {
		for _, second := range []string{filepath.Join(root, "out", "file"), filepath.Join(root, "out", "file", "child.txt"), filepath.Join(root, "out", "FILE", "child.txt")} {
			first := filepath.Join(root, "out", "file")
			if second == first {
				second = strings.ToUpper(first)
			}
			if reverse {
				first, second = second, first
			}
			result := e.PlanFileTargets([]string{a, b}, func(file string) (string, string) {
				if file == a {
					return first, ""
				}
				return second, ""
			})
			if len(result.Operations) != 1 || result.Operations[0].OldPath != a || len(result.Collisions) != 1 {
				t.Fatalf("file namespace collision missed: %+v", result)
			}
		}
	}
}

func TestFileTargetsRejectRelativeOrUncleanTargets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(nil, fs.NewAdapter())
	for _, target := range []string{"relative.txt", "", root + string(filepath.Separator) + ".." + string(filepath.Separator) + "other.txt"} {
		result := e.PlanFileTargets([]string{source}, func(string) (string, string) { return target, "" })
		if len(result.Operations) != 0 || len(result.Skipped) != 1 {
			t.Fatalf("invalid target admitted: %+v", result)
		}
	}
	result := e.PlanNames([]string{source}, func(string) (string, string) { return "escape/other.txt", "" })
	if len(result.Operations) != 0 {
		t.Fatal("ordinary basename planner now admits directory changes")
	}
}
