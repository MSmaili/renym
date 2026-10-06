//go:build linux || darwin || windows

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

func TestOrganizationRunDiscoveryAndUndoRequireLiveOriginalRoot(t *testing.T) {
	for _, change := range []string{"missing", "replaced"} {
		t.Run(change, func(t *testing.T) {
			service, _, plan, input, output := organizationApplyFixture(t, "one.txt")
			applied, err := service.Execute(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(input, input+"-original"); err != nil {
				t.Fatal(err)
			}
			if change == "replaced" {
				if err := os.Mkdir(input, 0700); err != nil {
					t.Fatal(err)
				}
			}
			runs, err := service.Runs(context.Background())
			if err != nil || len(runs) != 1 || runs[0].ID != applied.HistoryID || runs[0].Path != input {
				t.Fatalf("discovery: %+v %v", runs, err)
			}
			for _, dryRun := range []bool{true, false} {
				result, err := service.UndoRun(context.Background(), applied.HistoryID, dryRun)
				if err == nil || result.HistoryID != applied.HistoryID {
					t.Fatalf("unsafe undo: %+v %v", result, err)
				}
			}
			requireFileBytes(t, filepath.Join(output, "year/month/one.txt"), "bytes-one.txt")
		})
	}
}

func TestOrganizationRunOrderingAcrossRenameSchema(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	organized, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(input, "New File.txt"), "arrival")
	renamed, err := service.Rename(context.Background(), Request{Path: input, Mode: "snake", Files: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UndoRun(context.Background(), organized.HistoryID, false); err == nil || !strings.Contains(err.Error(), "latest eligible") {
		t.Fatalf("out-of-order undo: %v", err)
	}
	if _, err := service.UndoRun(context.Background(), renamed.HistoryID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UndoRun(context.Background(), organized.HistoryID, false); err != nil {
		t.Fatal(err)
	}
	entry, err := store.FindRun(organized.HistoryID)
	if err != nil || entry.State != history.OrganizationUndone {
		t.Fatalf("audit: %+v %v", entry, err)
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "bytes-one.txt")
	requireFileBytes(t, filepath.Join(input, "New File.txt"), "arrival")
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("owned root retained")
	}
}

func TestOrganizationUndoRejectsJournalOriginMismatch(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	applied, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := store.Latest(input)
	if err != nil {
		t.Fatal(err)
	}
	entry.Organization.SourceSnapshot.Identity = "different"
	if err := store.Update(input, applied.HistoryID, *entry); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Undo(context.Background(), input, false); err == nil {
		t.Fatal("malformed origin accepted")
	}
	put(t, filepath.Join(input, "New File.txt"), "arrival")
	if _, err := service.Rename(context.Background(), Request{Path: input, Mode: "snake", Files: true}); err == nil {
		t.Fatal("rename buried invalid organization history")
	}
	requireFileBytes(t, filepath.Join(output, "year/month/one.txt"), "bytes-one.txt")
	requireFileBytes(t, filepath.Join(input, "New File.txt"), "arrival")
}

func TestOrganizationRunAPIErrorsAndCancellation(t *testing.T) {
	service, _, plan, _, _ := organizationApplyFixture(t, "one.txt")
	applied, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "../escape.json", "missing.json"} {
		if _, err := service.UndoRun(context.Background(), id, false); err == nil {
			t.Fatalf("accepted ID %q", id)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Runs(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := service.UndoRun(ctx, applied.HistoryID, false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	unavailable := NewService(fs.NewAdapter(), nil)
	if _, err := unavailable.Runs(context.Background()); err == nil {
		t.Fatal("missing catalog accepted")
	}
	if _, err := unavailable.UndoRun(context.Background(), applied.HistoryID, false); err == nil {
		t.Fatal("missing catalog accepted")
	}
}

func TestNormalBuildKeepsOrganizationApplyGate(t *testing.T) {
	if organizationApplyEnabled {
		t.Skip("acceptance build")
	}
	input, output, policy := organizationFixture(t)
	put(t, filepath.Join(input, "one.txt"), "bytes")
	put(t, policy, movePreset(output, ""))
	service := NewService(nil, forbiddenStore{})
	if _, err := service.Rename(context.Background(), Request{Path: input, TemplatePath: policy}); !errors.Is(err, ErrOrganizationPreviewOnly) {
		t.Fatalf("public apply gate: %v", err)
	}
	requireFileBytes(t, filepath.Join(input, "one.txt"), "bytes")
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("gated apply created output")
	}
}

func TestNormalServiceUndoGateStillReportsRecovery(t *testing.T) {
	service, store, plan, input, output := organizationApplyFixture(t, "one.txt")
	applied, err := service.Execute(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	service.organizationEnabled = false
	preview, err := service.Undo(context.Background(), input, true)
	if err != nil || len(preview.Plan.Operations) != 1 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := service.Undo(context.Background(), input, false); !errors.Is(err, ErrOrganizationPreviewOnly) {
		t.Fatalf("undo gate: %v", err)
	}
	entry, err := store.Latest(input)
	if err != nil {
		t.Fatal(err)
	}
	entry.State = history.Pending
	if err := store.Update(input, applied.HistoryID, *entry); err != nil {
		t.Fatal(err)
	}
	result, err := service.Undo(context.Background(), input, false)
	if err == nil || !result.RequiresReconciliation {
		t.Fatalf("uncertainty hidden by gate: %+v %v", result, err)
	}
	requireFileBytes(t, filepath.Join(output, "year/month/one.txt"), "bytes-one.txt")
}
