package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/history"
)

func filenamePreset(expression string) string {
	return "version=1\nname='Filename pattern'\n[[rules]]\nid='all'\n[rules.rename]\nfilename='" + expression + "'\n"
}

func TestFilenameDatesPreviewHistoryAndUndo(t *testing.T) {
	root, store, service, req := setup(t)
	old := filepath.Join(root, "Screenshot One.PNG")
	put(t, old, "screenshot bytes")
	modified := time.Date(2026, 10, 5, 1, 2, 3, 0, time.FixedZone("fixture", 2*3600))
	if err := os.Chtimes(old, modified, modified); err != nil {
		t.Fatal(err)
	}
	req = withPreset(t, req, filenamePreset(`shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}`))
	req.DryRun = true
	preview, err := service.Plan(context.Background(), req)
	if err != nil || len(preview.Result.Operations) != 1 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	target := filepath.Join(root, "shot_2026-10-04_230203_001.png")
	if preview.Result.Operations[0].NewPath != target {
		t.Fatalf("UTC/time/extension/index wrong: %+v", preview.Result)
	}
	if _, err := service.Execute(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("preview created history: %v", err)
	}
	req.DryRun = false
	apply, err := service.Plan(context.Background(), req)
	if err != nil || apply.Result.Operations[0] != preview.Result.Operations[0] {
		t.Fatalf("apply differs from same snapshot preview: %+v, %v", apply, err)
	}
	// Compiled rules/targets are frozen; apply must never reload the policy.
	put(t, req.TemplatePath, "version=999")
	if _, err := service.Execute(context.Background(), apply); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, target, "screenshot bytes")
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(entry.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "filename") || !strings.Contains(string(encoded), "file.modified") || strings.Contains(string(encoded), "999") {
		t.Fatalf("history lost original spec: %s", encoded)
	}
	if _, err := service.Undo(context.Background(), root, true); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, target, "screenshot bytes")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, old, "screenshot bytes")
}

func TestFilenameIndexCountsFirstMatchesBeforeNoopsAndConflicts(t *testing.T) {
	root, _, service, req := setup(t)
	for _, name := range []string{"01.txt", "02.txt", "B File.txt", "C File.txt", "D File.txt", "Z File.md"} {
		put(t, filepath.Join(root, name), name)
	}
	req = withPreset(t, req, `version=1
[[rules]]
id='text'
[rules.match]
glob=['01.txt', 'B*', 'C*', 'D*']
extensions=['.txt']
[rules.rename]
filename='${index | pad(2)}${file.ext}'
[[rules]]
id='markdown'
[rules.match]
extensions=['.md']
[rules.rename]
filename='note_${index}${file.ext}'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Matches) != 5 || len(plan.Result.Operations) != 3 {
		t.Fatalf("index plan: %+v, %v", plan, err)
	}
	for i, want := range []int64{1, 2, 3, 4, 1} {
		if plan.Matches[i].Index != want {
			t.Fatalf("per-rule ordinals: %+v", plan.Matches)
		}
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for target, bytes := range map[string]string{"01.txt": "01.txt", "02.txt": "02.txt", "B File.txt": "B File.txt", "03.txt": "C File.txt", "04.txt": "D File.txt", "note_1.md": "Z File.md"} {
		bytesAt(t, filepath.Join(root, target), bytes)
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"01.txt", "02.txt", "B File.txt", "C File.txt", "D File.txt", "Z File.md"} {
		bytesAt(t, filepath.Join(root, name), name)
	}
}

func TestFilenameDirectoryMissingSizeSkipsWithoutFallbackButClaimsOrdinal(t *testing.T) {
	root, _, service, req := setup(t)
	if err := os.Mkdir(filepath.Join(root, "A Dir"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "B File.txt"), "bytes")
	req = withPreset(t, req, `version=1
[selection]
kind='both'
[[rules]]
id='first'
[rules.rename]
filename='${file.size}_${index}${file.ext}'
[[rules]]
id='fallback'
[rules.rename]
mode='snake'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Matches) != 2 || plan.Matches[1].Index != 2 || len(plan.Result.Skipped) != 1 || !strings.Contains(plan.Result.Skipped[0].Reason, "file.size is unavailable") {
		t.Fatalf("directory size/fallback/index wrong: %+v, %v", plan, err)
	}
	if plan.Result.Operations[0].NewPath != filepath.Join(root, "5_2.txt") {
		t.Fatalf("file size context: %+v", plan.Result)
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "5_2.txt"), "bytes")
	if _, err := os.Stat(filepath.Join(root, "A Dir")); err != nil {
		t.Fatal("failed render moved the directory")
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "B File.txt"), "bytes")
}

func TestFilenameRecursiveDirectoriesKeepLexicalIndicesAndPhysicalUndoOrder(t *testing.T) {
	for _, kind := range []string{"both", "directories"} {
		t.Run(kind, func(t *testing.T) {
			root, _, service, req := setup(t)
			original := filepath.Join(root, "Parent Dir.v1", "Child Dir", "Some File.txt")
			put(t, original, "nested")
			source := "version=1\n[selection]\nkind='" + kind + "'\nrecursive=true\n[[rules]]\nid='all'\n[rules.rename]\nfilename='${file.stem | snake}_${index | pad(2)}${file.ext}'\n"
			req = withPreset(t, req, source)
			plan, err := service.Plan(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			count := 3
			if kind == "directories" {
				count = 2
			}
			if len(plan.Matches) != count || len(plan.Result.Operations) != count || plan.Matches[0].Path != filepath.Join(root, "Parent Dir.v1") || plan.Matches[0].Index != 1 {
				t.Fatalf("lexical original evaluation lost: %+v", plan)
			}
			if plan.Result.Operations[0].OldPath == plan.Matches[0].Path {
				t.Fatal("physical execution sorted by index instead of deepest-first")
			}
			if _, err := service.Execute(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			name := "some_file_03.txt"
			if kind == "directories" {
				name = "Some File.txt"
			}
			bytesAt(t, filepath.Join(root, "parent_dir_v_1_01", "child_dir_02", name), "nested")
			if _, err := service.Undo(context.Background(), root, true); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Undo(context.Background(), root, false); err != nil {
				t.Fatal(err)
			}
			bytesAt(t, original, "nested")
		})
	}
}

func TestInvalidFilenameConfigBeforeDiscoveryAndNoMutation(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "Some File.txt")
	put(t, old, "keep")
	service := NewService(nil, forbiddenStore{})
	for _, expression := range []string{`${env.HOME}`, `${file.modified}`, `${index | pad(010)}`, `${file.name | shell}`} {
		req := withPreset(t, Request{Path: filepath.Join(root, "missing")}, filenamePreset(expression))
		if _, err := service.Rename(context.Background(), req); err == nil || !strings.Contains(err.Error(), "rename.filename") {
			t.Fatalf("bad renderer didn't precede discovery: %v", err)
		}
	}
	bytesAt(t, old, "keep")
}

func TestInvalidRenderedNamesSkipAndCannotReachExecution(t *testing.T) {
	for _, expression := range []string{"../escape", `bad\name`, "", strings.Repeat("x", 256)} {
		t.Run(expression, func(t *testing.T) {
			if expression == "" {
				expression = `${file.ext}`
			}
			root, store, service, req := setup(t)
			old := filepath.Join(root, "Some File")
			put(t, old, "keep")
			req = withPreset(t, req, filenamePreset(expression))
			result, err := service.Rename(context.Background(), req)
			if err != nil || len(result.Plan.Operations) != 0 || len(result.Plan.Skipped) != 1 {
				t.Fatalf("invalid rendered path escaped: %+v, %v", result, err)
			}
			if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
				t.Fatalf("render skip created history: %v", err)
			}
			bytesAt(t, old, "keep")
		})
	}
}

func TestRenderedTargetStillUsesNativeValidationAndNoOverwrite(t *testing.T) {
	root, _, service, req := setup(t)
	old, target := filepath.Join(root, "Some File.txt"), filepath.Join(root, "new.txt")
	put(t, old, "source")
	req = withPreset(t, req, filenamePreset("new.txt"))
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	put(t, target, "occupied after planning")
	if _, err := service.Execute(context.Background(), plan); err == nil {
		t.Fatal("rendered plan overwrote occupied target")
	}
	bytesAt(t, old, "source")
	bytesAt(t, target, "occupied after planning")
	if runtime.GOOS == "windows" {
		req = withPreset(t, req, filenamePreset("CON.txt"))
		req.Path = old
		plan, err = service.Plan(context.Background(), req)
		if err != nil || len(plan.Result.Operations) != 0 || len(plan.Result.Skipped) != 1 {
			t.Fatalf("renderer bypassed Windows reserved-name policy: %+v, %v", plan, err)
		}
	}
}

func TestFilenameIdentityPreservesDotfilesAndDoesNotReparseNames(t *testing.T) {
	root, store, service, req := setup(t)
	for _, name := range []string{".env", "${index}.txt", "archive.tar.gz"} {
		put(t, filepath.Join(root, name), name)
	}
	req = withPreset(t, req, filenamePreset(`${file.stem}${file.ext}`))
	result, err := service.Rename(context.Background(), req)
	if err != nil || len(result.Plan.Operations) != 0 {
		t.Fatalf("identity didn't preserve basenames: %+v, %v", result, err)
	}
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("identity wrote history: %v", err)
	}
	for _, name := range []string{".env", "${index}.txt", "archive.tar.gz"} {
		bytesAt(t, filepath.Join(root, name), name)
	}
}

func TestFilenameIndicesExcludeIgnoredProtectedUnmatchedAndUnsupportedEntries(t *testing.T) {
	root, _, service, req := setup(t)
	put(t, filepath.Join(root, "B File.txt"), "selected")
	put(t, filepath.Join(root, "C Unmatched.txt"), "unmatched")
	put(t, filepath.Join(root, ".git", "B Ignored.txt"), "ignored")
	source := "version=1\n[selection]\nkind='both'\nrecursive=true\n[[rules]]\nid='selected'\n[rules.match]\nglob=['B*']\n[rules.rename]\nfilename='${index}_${file.name}'\n"
	req = withPreset(t, req, source)
	req.TemplatePath = filepath.Join(root, "A Config", "B Preset.toml")
	put(t, req.TemplatePath, source)
	// When available, an unsupported source also sorts before the selected file.
	if err := os.Symlink(filepath.Join(root, "B File.txt"), filepath.Join(root, "A Link")); err != nil {
		t.Logf("symlink fixture unavailable: %v", err)
	}
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Matches) != 1 || plan.Matches[0].Index != 1 || len(plan.Result.Operations) != 1 || plan.Result.Operations[0].NewPath != filepath.Join(root, "1_B File.txt") {
		t.Fatalf("filtered entries claimed ordinals: %+v, %v", plan, err)
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "B File.txt"), "selected")
	bytesAt(t, filepath.Join(root, "C Unmatched.txt"), "unmatched")
	bytesAt(t, req.TemplatePath, source)
}
