package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

type faultStore struct {
	history.Store
	saveErr error
	hook    func(history.Entry) error
}

func (s *faultStore) Save(root string, entry history.Entry) (string, error) {
	if s.saveErr != nil {
		return "", s.saveErr
	}
	return s.Store.Save(root, entry)
}
func (s *faultStore) Update(root, id string, entry history.Entry) error {
	if s.hook != nil {
		if err := s.hook(entry); err != nil {
			return err
		}
	}
	return s.Store.Update(root, id, entry)
}

type forbiddenStore struct{}

func (forbiddenStore) Directory() string { return "" }

func (forbiddenStore) Save(string, history.Entry) (string, error) { panic("preview wrote history") }
func (forbiddenStore) Latest(string) (*history.Entry, error)      { panic("preview read history") }
func (forbiddenStore) Update(string, string, history.Entry) error { panic("preview updated history") }
func (forbiddenStore) DeleteEntry(string, string) error           { panic("preview deleted history") }

func setup(t *testing.T) (string, *history.GlobalStore, *Service, Request) {
	t.Helper()
	root := t.TempDir()
	adapter := fs.NewAdapter()
	store := history.NewStore(filepath.Join(t.TempDir(), "state"), adapter)
	return root, store, NewService(adapter, store), Request{Path: root, Mode: "snake", Files: true}
}
func put(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func bytesAt(t *testing.T, name, text string) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil || string(data) != text {
		t.Fatalf("%s: got %q, %v; want %q", name, data, err, text)
	}
}

func TestPreviewAndNoopNeverAccessHistory(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "Hello World.txt"), "original")
	service := NewService(nil, forbiddenStore{})
	req := Request{Path: root, Mode: "snake", Files: true, DryRun: true}
	result, err := service.Rename(context.Background(), req)
	if err != nil || len(result.Plan.Operations) != 1 || len(result.Execution.Completed) != 0 {
		t.Fatalf("preview: %+v, %v", result, err)
	}
	bytesAt(t, filepath.Join(root, "Hello World.txt"), "original")
	noop := t.TempDir()
	put(t, filepath.Join(noop, "normal.txt"), "unchanged")
	req.Path, req.DryRun = noop, false
	if _, err := service.Rename(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewLeavesPreviousUndoAvailable(t *testing.T) {
	root, store, service, req := setup(t)
	put(t, filepath.Join(root, "Hello World.txt"), "first")
	first, err := service.Rename(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "Another File.txt"), "preview")
	req.DryRun = true
	if _, err := service.Rename(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != first.HistoryID {
		t.Fatal("preview replaced latest successful history")
	}
	if _, err := service.Undo(context.Background(), root, true); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "hello_world.txt"), "first")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "Hello World.txt"), "first")
	bytesAt(t, filepath.Join(root, "Another File.txt"), "preview")
}

func TestRecursiveDirectoryUndoReversesPhysicalSteps(t *testing.T) {
	root, store, service, req := setup(t)
	original := filepath.Join(root, "Parent Dir.v1", "Child Dir", "Some File.txt")
	put(t, original, "nested bytes")
	req.Recursive, req.Directories = true, true
	result, err := service.Rename(context.Background(), req)
	if err != nil || len(result.Execution.Completed) != 3 {
		t.Fatalf("rename: %+v, %v", result, err)
	}
	bytesAt(t, filepath.Join(root, "parent_dir_v_1", "child_dir", "some_file.txt"), "nested bytes")
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Operations[0].Old != original {
		t.Fatal("history lost physical execution order")
	}
	if _, err := service.Undo(context.Background(), root, true); err != nil {
		t.Fatalf("nested preview failed: %v", err)
	}
	bytesAt(t, filepath.Join(root, "parent_dir_v_1", "child_dir", "some_file.txt"), "nested bytes")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, original, "nested bytes")
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("undo history still present: %v", err)
	}
}

func TestRequiredHistoryFailurePreventsMutation(t *testing.T) {
	root, store, _, req := setup(t)
	old := filepath.Join(root, "Hello World.txt")
	put(t, old, "original")
	service := NewService(nil, &faultStore{Store: store, saveErr: errors.New("disk full")})
	if _, err := service.Rename(context.Background(), req); err == nil {
		t.Fatal("required history failure ignored")
	}
	bytesAt(t, old, "original")
	req.SkipHistory = true
	if _, err := service.Rename(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "hello_world.txt"), "original")
}

func TestPartialFailureHistoryOnlyContainsCompletedSteps(t *testing.T) {
	root, store, _, req := setup(t)
	for _, name := range []string{"A File.txt", "B File.txt", "C File.txt"} {
		put(t, filepath.Join(root, name), name)
	}
	faults := &faultStore{Store: store, hook: func(entry history.Entry) error {
		if entry.State == history.Pending && len(entry.Operations) == 1 {
			put(t, filepath.Join(root, "b_file.txt"), "intruder")
		}
		return nil
	}}
	service := NewService(nil, faults)
	result, err := service.Rename(context.Background(), req)
	if err == nil || len(result.Execution.Completed) != 1 || result.Execution.Failed == nil || len(result.Execution.Unattempted) != 1 {
		t.Fatalf("wrong partial result: %+v, %v", result, err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	if entry.State != history.Partial || len(entry.Operations) != 1 || len(entry.Intent) != 3 || entry.Failed == nil || len(entry.Unattempted) != 1 {
		t.Fatalf("untruthful history: %+v", entry)
	}
	if entry.Operations[0].ID != 1 || entry.Failed.Operation.ID != 2 || entry.Unattempted[0].ID != 3 {
		t.Fatalf("operation IDs changed between intent and outcome: %+v", entry)
	}
	bytesAt(t, filepath.Join(root, "a_file.txt"), "A File.txt")
	bytesAt(t, filepath.Join(root, "B File.txt"), "B File.txt")
	bytesAt(t, filepath.Join(root, "C File.txt"), "C File.txt")
	bytesAt(t, filepath.Join(root, "b_file.txt"), "intruder")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "A File.txt"), "A File.txt")
	bytesAt(t, filepath.Join(root, "b_file.txt"), "intruder")
}

func TestCheckpointFailureStopsAndRetainsUncertainIntent(t *testing.T) {
	root, store, _, req := setup(t)
	put(t, filepath.Join(root, "A File.txt"), "A")
	put(t, filepath.Join(root, "B File.txt"), "B")
	service := NewService(nil, &faultStore{Store: store, hook: func(history.Entry) error { return errors.New("checkpoint failed") }})
	result, err := service.Rename(context.Background(), req)
	if err == nil || len(result.Execution.Completed) != 1 || len(result.Execution.Unattempted) != 1 {
		t.Fatalf("checkpoint result: %+v, %v", result, err)
	}
	entry, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	if entry.State != history.Pending || len(entry.Intent) != 2 || len(entry.Operations) != 0 {
		t.Fatalf("uncertain intent lost: %+v", entry)
	}
	bytesAt(t, filepath.Join(root, "a_file.txt"), "A")
	bytesAt(t, filepath.Join(root, "B File.txt"), "B")
	if _, err := service.Undo(context.Background(), root, false); err == nil {
		t.Fatal("uncertain history replayed")
	}
	if _, err := service.Rename(context.Background(), req); err == nil {
		t.Fatal("new apply ignored uncertain journal")
	}
}

func TestUndoRefusesLegacyIntentAndChangedFiles(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		root, store, service, _ := setup(t)
		new := filepath.Join(root, "new.txt")
		put(t, new, "keep")
		_, err := store.Save(root, history.Entry{Timestamp: time.Now(), Operations: []history.Operation{{Old: filepath.Join(root, "old.txt"), New: new}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Undo(context.Background(), root, false); err == nil {
			t.Fatal("legacy intent treated as completion")
		}
		bytesAt(t, new, "keep")
	})
	t.Run("changed", func(t *testing.T) {
		root, store, service, req := setup(t)
		put(t, filepath.Join(root, "Hello World.txt"), "original")
		if _, err := service.Rename(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		new := filepath.Join(root, "hello_world.txt")
		put(t, new, "edited contents")
		if _, err := service.Undo(context.Background(), root, true); !errors.Is(err, fs.ErrStalePlan) {
			t.Fatalf("stale undo preview accepted: %v", err)
		}
		before, err := store.Latest(root)
		if err != nil || before.State != history.Complete {
			t.Fatalf("preview changed history: %+v, %v", before, err)
		}
		if _, err := service.Undo(context.Background(), root, false); !errors.Is(err, fs.ErrStalePlan) {
			t.Fatalf("stale undo accepted: %v", err)
		}
		bytesAt(t, new, "edited contents")
		entry, err := store.Latest(root)
		if err != nil || entry.State != history.PartialUndo || entry.Undone != 0 {
			t.Fatalf("failed undo history lost: %+v, %v", entry, err)
		}
	})
}

func TestPartialUndoResumesWithoutReplayingRestoredSteps(t *testing.T) {
	root, store, service, req := setup(t)
	put(t, filepath.Join(root, "A File.txt"), "A")
	put(t, filepath.Join(root, "B File.txt"), "B")
	if _, err := service.Rename(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "A File.txt"), "occupied")
	result, err := service.Undo(context.Background(), root, false)
	if err == nil || len(result.Execution.Completed) != 1 {
		t.Fatalf("partial undo: %+v, %v", result, err)
	}
	entry, err := store.Latest(root)
	if err != nil || entry.State != history.PartialUndo || entry.Undone != 1 {
		t.Fatalf("wrong undo progress: %+v, %v", entry, err)
	}
	bytesAt(t, filepath.Join(root, "B File.txt"), "B")
	bytesAt(t, filepath.Join(root, "A File.txt"), "occupied")
	if err := os.Remove(filepath.Join(root, "A File.txt")); err != nil {
		t.Fatal(err)
	} // Remove only this test's injected blocker.
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "A File.txt"), "A")
	bytesAt(t, filepath.Join(root, "B File.txt"), "B")
}

func TestStalePlanAndCancellationDoNotMutate(t *testing.T) {
	root, store, service, req := setup(t)
	old := filepath.Join(root, "Hello World.txt")
	put(t, old, "original")
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	put(t, old, "modified")
	if _, err := service.Execute(context.Background(), plan); !errors.Is(err, fs.ErrStalePlan) {
		t.Fatalf("stale apply accepted: %v", err)
	}
	bytesAt(t, old, "modified")
	if _, err := store.Latest(root); !errors.Is(err, history.ErrNoHistory) {
		t.Fatalf("zero-completion history became undoable: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Plan(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPreflightFailurePreservesIntentOperationIDs(t *testing.T) {
	root, _, service, req := setup(t)
	put(t, filepath.Join(root, "A File.txt"), "A")
	put(t, filepath.Join(root, "B File.txt"), "B")
	put(t, filepath.Join(root, "C File.txt"), "C")
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "b_file.txt"), "occupied")
	result, err := service.Execute(context.Background(), plan)
	if err == nil || len(result.Execution.Completed) != 0 || result.Execution.Failed == nil || result.Execution.Failed.Operation.ID != 2 || len(result.Execution.Unattempted) != 2 {
		t.Fatalf("wrong preflight outcome: %+v, %v", result, err)
	}
	if result.Execution.Unattempted[0].ID != 1 || result.Execution.Unattempted[1].ID != 3 {
		t.Fatal("preflight failure changed unattempted operation IDs")
	}
	bytesAt(t, filepath.Join(root, "A File.txt"), "A")
	bytesAt(t, filepath.Join(root, "B File.txt"), "B")
	bytesAt(t, filepath.Join(root, "C File.txt"), "C")
}

func TestUndoCheckpointFailureDoesNotReplayUncertainSteps(t *testing.T) {
	root, store, service, req := setup(t)
	put(t, filepath.Join(root, "A File.txt"), "A")
	put(t, filepath.Join(root, "B File.txt"), "B")
	if _, err := service.Rename(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	service = NewService(nil, &faultStore{Store: store, hook: func(entry history.Entry) error {
		if entry.State == history.Undoing && entry.Undone > 0 {
			return errors.New("undo checkpoint failed")
		}
		return nil
	}})
	result, err := service.Undo(context.Background(), root, false)
	if err == nil || len(result.Execution.Completed) != 1 {
		t.Fatalf("wrong undo checkpoint outcome: %+v, %v", result, err)
	}
	entry, err := store.Latest(root)
	if err != nil || entry.State != history.Undoing || entry.Undone != 0 {
		t.Fatalf("uncertain undo intent lost: %+v, %v", entry, err)
	}
	bytesAt(t, filepath.Join(root, "B File.txt"), "B")
	bytesAt(t, filepath.Join(root, "a_file.txt"), "A")
	if _, err := service.Undo(context.Background(), root, false); err == nil {
		t.Fatal("uncertain undo replayed")
	}
}

func TestHistoryDirectoryCannotRenameItsOwnJournal(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "My History")
	put(t, filepath.Join(state, "History File.json"), "keep history")
	put(t, filepath.Join(root, "Hello World.txt"), "rename this")
	store := history.NewStore(state, fs.NewAdapter())
	service := NewService(nil, store)
	req := Request{Path: root, Mode: "snake", Files: true, Directories: true, Recursive: true, DryRun: true}
	plan, err := service.Plan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Result.Operations) != 1 || len(plan.Result.Skipped) != 2 {
		t.Fatalf("journal paths not protected: %+v", plan.Result)
	}
	bytesAt(t, filepath.Join(state, "History File.json"), "keep history")
}

func TestFutureHistoryThroughAnAliasProtectsItsAncestors(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "Dot Files")
	config := filepath.Join(parent, "Config Folder")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(config, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store := history.NewStore(filepath.Join(alias, "renym"), fs.NewAdapter())
	service := NewService(nil, store)
	plan, err := service.Plan(context.Background(), Request{Path: root, Mode: "snake", Directories: true, Recursive: true, DryRun: true})
	if err != nil || len(plan.Result.Operations) != 0 || len(plan.Result.Skipped) != 2 {
		t.Fatalf("aliased journal ancestor unprotected: %+v, %v", plan.Result, err)
	}
}
