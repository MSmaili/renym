package history

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/fs"
)

func TestRunDiscoverySurvivesMissingInputAndKeepsOrganizationAudit(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	entry := Entry{SchemaVersion: OrganizationSchemaVersion, State: OrganizationUndone, Timestamp: time.Now().Add(-time.Hour), Organization: testOrganizationAudit(root)}
	id, err := store.Save(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if _, err := store.Save(root, Entry{SchemaVersion: SchemaVersion, State: Complete, Timestamp: time.Now().Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.dirHistoryPath("root"), id)); err != nil {
		t.Fatal("organization audit pruned")
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	runs, err := store.Runs()
	if err != nil || len(runs) != 3 {
		t.Fatalf("discovery: %+v %v", runs, err)
	}
	found, err := store.FindRun(id)
	if err != nil || found.Path != root || found.State != OrganizationUndone {
		t.Fatalf("run lost after input disappears: %+v %v", found, err)
	}
	for _, id := range []string{"../escape.json", `a\b.json`, "", "/abs.json"} {
		if _, err := store.FindRun(id); err == nil {
			t.Fatalf("invalid run ID: %q", id)
		}
	}
}

func TestRunDiscoveryExposesCorruptionAndAmbiguity(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	id, err := store.Save(root, Entry{Timestamp: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.dirHistoryPath("root"), id))
	if err != nil {
		t.Fatal(err)
	}
	other := store.dirHistoryPath("other")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, id), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FindRun(id); err == nil {
		t.Fatal("duplicate ID was arbitrarily resolved")
	}
	if err := os.WriteFile(filepath.Join(other, "corrupt.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	runs, err := store.Runs()
	if err != nil {
		t.Fatal(err)
	}
	corrupt := false
	for _, run := range runs {
		if run.ID == "corrupt.json" && run.Error != "" {
			corrupt = true
		}
	}
	if !corrupt {
		t.Fatal("corrupt record disappeared")
	}
}

func TestOrganizationUncertaintyBlocksNewerRunsAndUndoneIsNotReplayable(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	entry := Entry{SchemaVersion: OrganizationSchemaVersion, State: Complete, Timestamp: time.Now().Add(-time.Hour), Organization: &Organization{Active: &OrganizationStep{Action: "move", OperationID: 1}}}
	id, err := store.Save(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(root, Entry{SchemaVersion: SchemaVersion, State: Complete, Timestamp: time.Now()}); err != nil {
		t.Fatal(err)
	}
	latest, err := store.Latest(root)
	if err != nil || latest.ID != id {
		t.Fatal("uncertain organization was buried")
	}
	entry.State, entry.Organization.Active = OrganizationUndone, nil
	entry.Organization = testOrganizationAudit(root)
	if err := store.Update(root, id, entry); err != nil {
		t.Fatal(err)
	}
	latest, err = store.Latest(root)
	if err != nil || latest.ID == id {
		t.Fatal("completed undo selected for replay")
	}
}

func testOrganizationAudit(root string) *Organization {
	snapshot := &fs.Snapshot{Identity: "root", Mode: os.ModeDir | 0700}
	return &Organization{SourceRoot: root, SourceSnapshot: snapshot, Bindings: []DirectoryBinding{{Request: fs.DirectoryRequest{Root: root, RootSnapshot: snapshot}, Snapshot: snapshot}}}
}

func TestRunDiscoveryEmptyStore(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing"), nil)
	if runs, err := store.Runs(); err != nil || len(runs) != 0 {
		t.Fatal(runs, err)
	}
	if _, err := store.FindRun("missing.json"); !errors.Is(err, ErrNoHistory) {
		t.Fatal(err)
	}
}
