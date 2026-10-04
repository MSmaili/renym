package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSameTimestampDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	stamp := time.Date(2026, 10, 4, 1, 2, 3, 0, time.UTC)
	one, err := store.Save(root, Entry{Timestamp: stamp, Command: "one"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := store.Save(root, Entry{Timestamp: stamp, Command: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("same-timestamp history overwrote another run")
	}
	for _, name := range []string{one, two} {
		if _, err := os.Stat(filepath.Join(store.dirHistoryPath("root"), name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVerifiedHistoryPreservesExecutionOrderAndCheckpoints(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	entry := Entry{SchemaVersion: SchemaVersion, State: Pending, Timestamp: time.Now(), Intent: []Operation{{Old: "a/b", New: "a/c"}, {Old: "a", New: "d"}}}
	id, err := store.Save(root, entry)
	if err != nil {
		t.Fatal(err)
	}
	entry.Operations, entry.State = entry.Intent, Complete
	if err := store.Update(root, id, entry); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Latest(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != id || loaded.State != Complete || loaded.Operations[0].Old != "a/b" {
		t.Fatalf("checkpoint/order lost: %+v", loaded)
	}
	if err := store.Update(root, "../escape.json", entry); err == nil {
		t.Fatal("unsafe record ID accepted")
	}
}

func TestCleanupRetainsUnresolvedHistory(t *testing.T) {
	root := t.TempDir()
	store, _ := newTestStore(t, &mockPathIdentifier{ids: map[string]string{root: "root"}})
	id, err := store.Save(root, Entry{SchemaVersion: SchemaVersion, State: Pending, Timestamp: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if _, err := store.Save(root, Entry{SchemaVersion: SchemaVersion, State: Complete, Timestamp: time.Now().Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.dirHistoryPath("root"), id)); err != nil {
		t.Fatalf("unresolved intent pruned: %v", err)
	}
	latest, err := store.Latest(root)
	if err != nil || latest.ID != id {
		t.Fatalf("older unresolved intent hidden by newer history: %+v, %v", latest, err)
	}
}
