//go:build linux || darwin || windows

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

func organizationApplyFixture(t *testing.T, files ...string) (*Service, *history.GlobalStore, Plan, string, string) {
	t.Helper()
	input, output, policy := organizationFixture(t)
	for _, name := range files {
		put(t, filepath.Join(input, name), "bytes-"+name)
	}
	put(t, policy, movePreset(output, "year/month"))
	store := history.NewStore(t.TempDir(), fs.NewAdapter())
	service := NewService(nil, store)
	service.organizationEnabled = true
	plan, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy})
	if err != nil {
		t.Fatal(err)
	}
	return service, store, plan, input, output
}

func requireFileBytes(t *testing.T, path, content string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Fatalf("%s: %q %v", path, data, err)
	}
}

func TestInternalOrganizationJournaledRoundTrip(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt", "two.txt")
	plan.Result.Operations[0].NewPath = filepath.Join(input, "tampered.txt")
	plan.DirectoriesToCreate = nil
	result, err := service.Execute(context.Background(), plan)
	if err != nil || len(result.Execution.Completed) != 2 || len(result.DirectoriesCreated) != 3 || result.HistoryID == "" || result.RequiresReconciliation {
		t.Fatalf("apply: %+v %v", result, err)
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		requireFileBytes(t, filepath.Join(output, "year/month", name), "bytes-"+name)
		if _, err := os.Lstat(filepath.Join(input, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("move retained origin")
		}
	}
	entry, err := store.Latest(input)
	if err != nil || entry.SchemaVersion != 2 || entry.State != history.Complete || len(entry.Operations) != 2 || entry.Organization.Active != nil {
		t.Fatalf("journal: %+v %v", entry, err)
	}
	found, err := store.FindRun(result.HistoryID)
	if err != nil || found.Path != input {
		t.Fatalf("run not discoverable from empty input: %+v %v", found, err)
	}
	before := *entry
	preview, err := service.Undo(context.Background(), input, true)
	if err != nil || len(preview.Plan.Operations) != 2 {
		t.Fatalf("undo preview: %+v %v", preview, err)
	}
	after, err := store.Latest(input)
	if err != nil || !reflect.DeepEqual(before, *after) {
		t.Fatal("undo preview changed journal")
	}
	undone, err := service.Undo(context.Background(), input, false)
	if err != nil || len(undone.Execution.Completed) != 2 || len(undone.DirectoriesRemoved) != 3 {
		t.Fatalf("undo: %+v %v", undone, err)
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		requireFileBytes(t, filepath.Join(input, name), "bytes-"+name)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned output retained: %v", err)
	}
	if _, err := store.Latest(input); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("undone journal still selected: %v", err)
	}
	found, err = store.FindRun(result.HistoryID)
	if err != nil || found.State != history.OrganizationUndone || found.Undone != 2 || found.Organization.Cleaned != 3 {
		t.Fatalf("undo audit lost: %+v %v", found, err)
	}
}

func TestInternalOrganizationKeepsPreexistingAndPopulatedDirectories(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	result, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(output, "year/month/user.txt"), "user bytes")
	undone, err := service.Undo(context.Background(), input, false)
	if err != nil || len(undone.DirectoriesRetained) != 3 || len(undone.DirectoriesRemoved) != 0 {
		t.Fatalf("populated cleanup: %+v %v", undone, err)
	}
	requireFileBytes(t, filepath.Join(output, "year/month/user.txt"), "user bytes")
	entry, err := store.FindRun(result.HistoryID)
	if err != nil || len(entry.Organization.Retained) != 3 {
		t.Fatal("retention reasons lost")
	}
	plan, err = service.Plan(context.Background(), plan.request)
	if err != nil || len(plan.organization.directories) != 0 {
		t.Fatalf("existing directories adopted: %v", err)
	}
	result, err = service.Execute(context.Background(), plan)
	if err != nil || len(result.DirectoriesCreated) != 0 {
		t.Fatalf("preexisting apply: %+v %v", result, err)
	}
	undone, err = service.Undo(context.Background(), input, false)
	if err != nil || len(undone.DirectoriesRemoved) != 0 {
		t.Fatal("preexisting directory removed")
	}
}

func TestInternalOrganizationRequiresHistoryAndValidatesBeforeCreation(t *testing.T) {
	for _, change := range []string{"skip-history", "missing-store", "source-change", "parent-replacement", "appearing-directory", "save-error"} {
		t.Run(change, func(t *testing.T) {
			service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
			switch change {
			case "skip-history":
				plan.request.SkipHistory = true
			case "missing-store":
				service.store = nil
			case "source-change":
				put(t, filepath.Join(input, "one.txt"), "changed length")
			case "parent-replacement":
				if err := os.Rename(input, input+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(input, 0700); err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(input, "one.txt"), "replacement")
			case "appearing-directory":
				if err := os.Mkdir(output, 0700); err != nil {
					t.Fatal(err)
				}
			case "save-error":
				service.store = &faultStore{Store: store, saveErr: errors.New("intent failure")}
			}
			result, err := service.Execute(context.Background(), plan)
			if err == nil || len(result.Execution.Completed) != 0 || len(result.DirectoriesCreated) != 0 {
				t.Fatalf("invalid apply mutated: %+v %v", result, err)
			}
			if change != "appearing-directory" {
				if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failure created output")
				}
			}
		})
	}
}

func TestInternalOrganizationDirectoryOnlyPartialRunCanUndo(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.store = &faultStore{Store: store, hook: func(entry history.Entry) error {
		if len(entry.Organization.Directories) == 1 && entry.Organization.Active == nil {
			cancel()
		}
		return nil
	}}
	result, err := service.Execute(ctx, plan)
	if !errors.Is(err, context.Canceled) || len(result.DirectoriesCreated) != 1 || len(result.Execution.Completed) != 0 || result.RequiresReconciliation {
		t.Fatalf("directory-only partial: %+v %v", result, err)
	}
	entry, err := store.Latest(input)
	if err != nil || entry.State != history.Partial || len(entry.Organization.Directories) != 1 || len(entry.Unattempted) != 1 {
		t.Fatalf("partial ownership lost: %+v %v", entry, err)
	}
	service.store = store
	undone, err := service.Undo(context.Background(), input, false)
	if err != nil || len(undone.DirectoriesRemoved) != 1 {
		t.Fatalf("directory-only undo: %+v %v", undone, err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned output retained")
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "bytes-one.txt")
}

func TestInternalOrganizationCheckpointFailureStopsAndRefusesReplay(t *testing.T) {
	for _, boundary := range []string{"before-mkdir", "after-mkdir", "after-move"} {
		t.Run(boundary, func(t *testing.T) {
			service, store, plan, input, output := organizationApplyFixture(t, "one.txt", "two.txt")
			service.store = &faultStore{Store: store, hook: func(entry history.Entry) error {
				org := entry.Organization
				fail := boundary == "before-mkdir" && org.Active != nil && org.Active.Action == "mkdir" || boundary == "after-mkdir" && len(org.Directories) == 1 && org.Active == nil || boundary == "after-move" && len(entry.Operations) == 1
				if fail {
					return errors.New("checkpoint failure")
				}
				return nil
			}}
			result, err := service.Execute(context.Background(), plan)
			if err == nil || !result.RequiresReconciliation {
				t.Fatalf("checkpoint failure hidden: %+v %v", result, err)
			}
			entry, err := store.Latest(input)
			if err != nil || entry.State != history.Pending {
				t.Fatalf("uncertain run finalized: %+v %v", entry, err)
			}
			service.store = store
			if _, err := service.Undo(context.Background(), input, false); err == nil {
				t.Fatal("uncertain run automatically undone")
			}
			switch boundary {
			case "before-mkdir":
				if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("mkdir ran before checkpoint")
				}
			case "after-mkdir":
				if len(result.DirectoriesCreated) != 1 || entry.Organization.Active == nil {
					t.Fatal("ownership failure omitted")
				}
			case "after-move":
				if len(result.Execution.Completed) != 1 || entry.Organization.Active == nil || entry.Organization.Active.Action != "move" {
					t.Fatal("completed move lost")
				}
			}
			requireFileBytes(t, filepath.Join(input, "two.txt"), "bytes-two.txt")
		})
	}
}

func TestInternalOrganizationPartialMoveAndUndoConflictResume(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt", "two.txt")
	service.store = &faultStore{Store: store, hook: func(entry history.Entry) error {
		if len(entry.Operations) == 1 && entry.State == history.Pending && entry.Organization.Active == nil {
			put(t, filepath.Join(input, "two.txt"), "changed source")
		}
		return nil
	}}
	result, err := service.Execute(context.Background(), plan)
	if !errors.Is(err, fs.ErrStalePlan) || len(result.Execution.Completed) != 1 || result.RequiresReconciliation {
		t.Fatalf("partial move: %+v %v", result, err)
	}
	service.store = store
	put(t, filepath.Join(input, "one.txt"), "new arrival")
	undo, err := service.Undo(context.Background(), input, false)
	if err == nil || len(undo.Execution.Completed) != 0 {
		t.Fatalf("undo overwrite: %+v %v", undo, err)
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "new arrival")
	entry, err := store.Latest(input)
	if err != nil || entry.State != history.PartialUndo {
		t.Fatal("undo failure not resumable")
	}
	if err := os.Rename(filepath.Join(input, "one.txt"), filepath.Join(input, "arrival.txt")); err != nil {
		t.Fatal(err)
	}
	undo, err = service.Undo(context.Background(), input, false)
	if err != nil || len(undo.Execution.Completed) != 1 {
		t.Fatalf("resume undo: %+v %v", undo, err)
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "bytes-one.txt")
	requireFileBytes(t, filepath.Join(input, "two.txt"), "changed source")
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial directories not cleaned")
	}
}

func TestInternalOrganizationMixedRulesMultipleDestinations(t *testing.T) {
	input, output, policy := organizationFixture(t)
	second := output + "-second"
	put(t, filepath.Join(input, "one.txt"), "one")
	put(t, filepath.Join(input, "two.png"), "two")
	put(t, filepath.Join(input, "THREE.md"), "three")
	put(t, policy, fmt.Sprintf(`version=1
[[rules]]
id='text'
[rules.match]
extensions=['.txt']
[rules.rename]
filename='renamed.txt'
[rules.move]
root=%q
directory='nested'
[[rules]]
id='image'
[rules.match]
extensions=['.png']
[rules.move]
root=%q
[[rules]]
id='remaining'
[rules.rename]
filename='three-renamed.md'
`, output, second))
	service := NewService(nil, history.NewStore(t.TempDir(), fs.NewAdapter()))
	service.organizationEnabled = true
	plan, err := service.Plan(context.Background(), Request{Path: input, TemplatePath: policy})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Execute(context.Background(), plan)
	if err != nil || len(result.Execution.Completed) != 3 {
		t.Fatalf("mixed: %+v %v", result, err)
	}
	requireFileBytes(t, filepath.Join(output, "nested/renamed.txt"), "one")
	requireFileBytes(t, filepath.Join(second, "two.png"), "two")
	requireFileBytes(t, filepath.Join(input, "three-renamed.md"), "three")
	if _, err := service.Undo(context.Background(), input, false); err != nil {
		t.Fatal(err)
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "one")
	requireFileBytes(t, filepath.Join(input, "two.png"), "two")
	requireFileBytes(t, filepath.Join(input, "THREE.md"), "three")
}

func TestInternalOrganizationUndoCheckpointFailuresNeverReplay(t *testing.T) {
	for _, boundary := range []string{"before-move", "after-move", "before-cleanup", "after-cleanup"} {
		t.Run(boundary, func(t *testing.T) {
			service, store, plan, input, _ := organizationApplyFixture(t, "one.txt")
			if _, err := service.Execute(context.Background(), plan); err != nil {
				t.Fatal(err)
			}
			service.store = &faultStore{Store: store, hook: func(entry history.Entry) error {
				org := entry.Organization
				if entry.State != history.Undoing {
					return nil
				}
				fail := boundary == "before-move" && org.Active != nil && org.Active.Action == "undo_move" || boundary == "after-move" && entry.Undone == 1 && org.Active == nil || boundary == "before-cleanup" && org.Active != nil && org.Active.Action == "rmdir" || boundary == "after-cleanup" && org.Cleaned == 1 && org.Active == nil
				if fail {
					return errors.New("undo checkpoint failure")
				}
				return nil
			}}
			result, err := service.Undo(context.Background(), input, false)
			if err == nil || !result.RequiresReconciliation {
				t.Fatalf("undo checkpoint uncertainty hidden: %+v %v", result, err)
			}
			entry, err := store.Latest(input)
			if err != nil || entry.State != history.Undoing {
				t.Fatalf("uncertain undo finalized: %+v %v", entry, err)
			}
			service.store = store
			if _, err := service.Undo(context.Background(), input, false); err == nil {
				t.Fatal("uncertain undo automatically resumed")
			}
			if boundary == "after-move" && len(result.Execution.Completed) != 1 {
				t.Fatal("restored file omitted from partial result")
			}
			if boundary == "after-cleanup" && len(result.DirectoriesRemoved) != 1 {
				t.Fatal("completed cleanup omitted from partial result")
			}
		})
	}
}

func TestInternalOrganizationCleanupKeepsReplacement(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.store = &faultStore{Store: store, hook: func(entry history.Entry) error {
		if len(entry.Organization.Directories) == 1 && entry.Organization.Active == nil {
			cancel()
		}
		return nil
	}}
	if _, err := service.Execute(ctx, plan); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := os.Rename(output, output+"-owned"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	service.store = store
	result, err := service.Undo(context.Background(), input, false)
	if err != nil || len(result.DirectoriesRemoved) != 0 || len(result.DirectoriesRetained) != 1 {
		t.Fatalf("replacement cleanup: %+v %v", result, err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal("replacement removed")
	}
	if _, err := os.Stat(output + "-owned"); err != nil {
		t.Fatal("relocated owned directory removed")
	}
}

func TestInternalOrganizationUndoRefusesChangedFileAndMalformedRecords(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	result, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(output, "year/month/one.txt"), "new destination bytes")
	if _, err := service.Undo(context.Background(), input, true); !errors.Is(err, fs.ErrStalePlan) {
		t.Fatalf("changed target accepted: %v", err)
	}
	entry, err := store.FindRun(result.HistoryID)
	if err != nil {
		t.Fatal(err)
	}
	entry.Operations[0].Move.OldRelative = "../escape.txt"
	if err := store.Update(input, entry.ID, *entry); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Undo(context.Background(), input, false); err == nil {
		t.Fatal("malformed relative path accepted")
	}
	requireFileBytes(t, filepath.Join(output, "year/month/one.txt"), "new destination bytes")
}

func TestOrganizationBindingNeverRecapturesRenderedSourceVersion(t *testing.T) {
	service, _, plan, input, _ := organizationApplyFixture(t, "one.txt")
	request := plan.request
	request.DryRun = true
	plan, err := service.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(input, "one.txt"), "changed after filename rendering")
	planner := organizationPlanner{service: service, root: input, plan: &organizationPlan{source: cloneValue(plan.sourceDirectory)}, sources: plan.proposalSources, seenBindings: map[string]bool{}, seenDirectories: map[string]string{}}
	if err := planner.addStep(context.Background(), plan.Result.Operations[0], 1); !errors.Is(err, fs.ErrStalePlan) {
		t.Fatalf("rendered source version was replaced: %v", err)
	}
}
