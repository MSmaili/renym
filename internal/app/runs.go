package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

func readyForNewRun(entry *history.Entry) error {
	if entry == nil || entry.SchemaVersion == 0 {
		return nil
	}
	if !history.VerifiedVersion(entry.SchemaVersion) || entry.State != history.Complete && entry.State != history.Partial {
		return errors.New("existing history requires reconciliation or completion of undo before another run")
	}
	if entry.SchemaVersion == history.OrganizationSchemaVersion {
		return validateOrganizationJournal(entry)
	}
	return nil
}

func (s *Service) Runs(ctx context.Context) ([]history.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	catalog, ok := s.store.(history.RunCatalog)
	if !ok {
		return nil, errors.New("history run discovery is unavailable")
	}
	runs, err := catalog.Runs()
	if err != nil {
		return runs, err
	}
	return runs, ctx.Err()
}

// Only the latest eligible run for a live, unchanged input root can be undone.
func (s *Service) UndoRun(ctx context.Context, id string, dryRun bool) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	catalog, ok := s.store.(history.RunCatalog)
	if !ok {
		return Result{}, errors.New("history run discovery is unavailable")
	}
	entry, err := catalog.FindRun(id)
	if err != nil {
		return Result{}, err
	}
	result := Result{HistoryID: id, SourcePath: entry.Path, Organization: entry.SchemaVersion == history.OrganizationSchemaVersion}
	root, err := fs.Capture(entry.Path)
	if err != nil {
		return result, err
	}
	if !root.Mode.IsDir() || root.Identity != entry.DirID {
		return result, fs.ErrStalePlan
	}
	latest, err := s.store.Latest(entry.Path)
	if err != nil {
		return result, err
	}
	if latest == nil || latest.ID != id {
		return result, fmt.Errorf("only the latest eligible run for %s can be undone", entry.Path)
	}
	return s.Undo(ctx, entry.Path, dryRun)
}
