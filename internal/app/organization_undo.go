package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

func (s *Service) undoOrganizationEntry(ctx context.Context, dryRun bool, entry *history.Entry) (Result, error) {
	result := Result{Organization: true, SourcePath: entry.Path, HistoryID: entry.ID}
	if err := validateOrganizationJournal(entry); err != nil {
		result.RequiresReconciliation = entry.State == history.Pending || entry.State == history.Undoing || entry.Organization != nil && entry.Organization.Active != nil
		return result, err
	}
	if _, err := fs.InspectDirectory(ctx, fs.DirectoryRequest{Root: entry.Organization.SourceRoot, RootSnapshot: entry.Organization.SourceSnapshot}); err != nil {
		return result, err
	}
	run := organizationRun{service: s, root: entry.Organization.SourceRoot, entry: *entry, result: result}
	steps := entry.Operations[:len(entry.Operations)-entry.Undone]
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		run.result.Plan.Operations = append(run.result.Plan.Operations, engine.RenameOp{OldPath: step.New, NewPath: step.Old})
	}
	if dryRun {
		run.result.DirectoryCleanupCandidates = organizationCleanupCandidates(entry.Organization)
		return run.result, run.previewOrganizationUndo(ctx, steps)
	}
	if !s.organizationEnabled {
		return run.result, ErrOrganizationPreviewOnly
	}
	if err := ctx.Err(); err != nil {
		return run.result, err
	}
	run.entry.State, run.entry.UndoFailed = history.Undoing, nil
	if err := run.checkpoint(); err != nil {
		return run.result, err
	}
	if err := run.reverseFiles(ctx, steps); err != nil {
		return run.finishUndo(err)
	}
	if err := run.cleanupDirectories(ctx); err != nil {
		return run.finishUndo(err)
	}
	return run.finishUndo(nil)
}

func (run *organizationRun) reverseFiles(ctx context.Context, steps []history.Operation) error {
	for i := len(steps) - 1; i >= 0; i-- {
		if err := run.reverseMove(ctx, steps[i]); err != nil {
			step := steps[i]
			run.entry.UndoFailed = &history.Failure{Operation: step, Error: err.Error()}
			run.result.Execution.Failed = &fs.Failure{Operation: fs.RenameOp{ID: step.ID, OldPath: step.New, NewPath: step.Old, Source: step.Source}, Err: err}
			for j := i - 1; j >= 0; j-- {
				remaining := steps[j]
				run.result.Execution.Unattempted = append(run.result.Execution.Unattempted, fs.RenameOp{ID: remaining.ID, OldPath: remaining.New, NewPath: remaining.Old, Source: remaining.Source})
			}
			return err
		}
	}
	return nil
}

func organizationCleanupCandidates(org *history.Organization) []string {
	var paths []string
	for i := len(org.Directories) - org.Cleaned - 1; i >= 0; i-- {
		paths = append(paths, directoryPath(org.Directories[i].DirectoryRequest))
	}
	return paths
}

func validateOrganizationJournal(entry *history.Entry) error {
	if entry == nil || entry.SchemaVersion != history.OrganizationSchemaVersion || entry.Organization == nil {
		return errors.New("unsupported organization journal")
	}
	org := entry.Organization
	if entry.State != history.Complete && entry.State != history.Partial && entry.State != history.PartialUndo || org.Active != nil {
		return errors.New("organization journal requires reconciliation")
	}
	if entry.Undone < 0 || entry.Undone > len(entry.Operations) || org.Cleaned < 0 || org.Cleaned > len(org.Directories) || !validOrganizationOrigin(entry) || len(org.Bindings) == 0 {
		return errors.New("invalid organization journal")
	}
	for _, step := range entry.Operations {
		if err := validateMoveRecord(step, org); err != nil {
			return err
		}
	}
	for _, owned := range org.Directories {
		if err := validateOwnedDirectory(owned); err != nil {
			return err
		}
	}
	return nil
}

func validOrganizationOrigin(entry *history.Entry) bool {
	org := entry.Organization
	return filepath.IsAbs(org.SourceRoot) && filepath.Clean(org.SourceRoot) == org.SourceRoot && org.SourceRoot == entry.Path && verifiedDirectorySnapshot(org.SourceSnapshot) && org.SourceSnapshot.Identity == entry.DirID
}

func validateOwnedDirectory(owned fs.OwnedDirectory) error {
	if owned.Snapshot == nil || owned.Snapshot.Identity == "" || !owned.Snapshot.Mode.IsDir() || owned.ParentSnapshot == nil || owned.RootSnapshot == nil || owned.RootSnapshot.Identity == "" || !filepath.IsAbs(owned.Root) {
		return errors.New("unverified directory ownership")
	}
	_, err := absoluteRelative(owned.Root, owned.Relative)
	return err
}

func validateMoveRecord(step history.Operation, org *history.Organization) error {
	req := step.Move
	if req == nil || step.Source == nil || step.Source.Identity == "" || !step.Source.Mode.IsRegular() || req.Source == nil || req.Source.Identity != step.Source.Identity || req.SourceRoot != org.SourceRoot || req.SourceDirectory == nil || req.SourceDirectory.Identity != org.SourceSnapshot.Identity || req.DestinationDirectory == nil || req.DestinationDirectory.Identity == "" {
		return errors.New("unverified move history")
	}
	if !verifiedDirectorySnapshot(req.SourceParent) || !verifiedDirectorySnapshot(req.DestinationParent) {
		return errors.New("move history lacks accepted parent identities")
	}
	old, err := absoluteRelative(req.SourceRoot, req.OldRelative)
	if err != nil {
		return err
	}
	new, err := absoluteRelative(req.DestinationRoot, req.NewRelative)
	if err != nil {
		return err
	}
	if old != step.Old || new != step.New || old == new {
		return errors.New("move paths disagree with rooted journal")
	}
	return nil
}

func verifiedDirectorySnapshot(snapshot *fs.Snapshot) bool {
	return snapshot != nil && snapshot.Identity != "" && snapshot.Mode.IsDir()
}

func absoluteRelative(root, relative string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", fs.ErrUnsafeMovePath
	}
	local, err := filepath.Localize(relative)
	if err != nil || local == "." {
		return "", fs.ErrUnsafeMovePath
	}
	return filepath.Join(root, local), nil
}

func reverseMoveRequest(step history.Operation) fs.MoveRequest {
	req := *step.Move
	req.SourceRoot, req.DestinationRoot = req.DestinationRoot, req.SourceRoot
	req.SourceDirectory, req.DestinationDirectory = req.DestinationDirectory, req.SourceDirectory
	req.OldRelative, req.NewRelative = req.NewRelative, req.OldRelative
	req.SourceParent, req.DestinationParent = req.DestinationParent, req.SourceParent
	req.Source = step.Source
	return req
}

func (r *organizationRun) previewOrganizationUndo(ctx context.Context, steps []history.Operation) error {
	if len(steps) == 0 {
		_, err := fs.InspectDirectory(ctx, fs.DirectoryRequest{Root: r.entry.Organization.SourceRoot, RootSnapshot: r.entry.Organization.SourceSnapshot})
		return err
	}
	if err := checkDirectoryBindings(ctx, r.entry.Organization.Bindings); err != nil {
		return err
	}
	for i := len(steps) - 1; i >= 0; i-- {
		move, err := fs.PrepareMove(ctx, reverseMoveRequest(steps[i]))
		if err != nil {
			return err
		}
		if err := move.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (r *organizationRun) reverseMove(ctx context.Context, step history.Operation) error {
	if err := r.checkMoveParents(ctx, reverseMoveRequest(step)); err != nil {
		return err
	}
	if r.service.protectsHistory(step.Old) || r.service.protectsHistory(step.New) {
		return errors.New("protected move undo path")
	}
	move, err := fs.PrepareMove(ctx, reverseMoveRequest(step))
	if err != nil {
		return err
	}
	defer move.Close()
	req := reverseMoveRequest(step)
	r.entry.Organization.Active = &history.OrganizationStep{Action: "undo_move", OperationID: step.ID, Move: &req}
	if err := r.checkpoint(); err != nil {
		return err
	}
	outcome, err := move.Execute(ctx)
	r.result.MoveOutcomes = append(r.result.MoveOutcomes, outcome)
	if outcome.Completed {
		r.result.Execution.Completed = append(r.result.Execution.Completed, fs.RenameOp{ID: step.ID, OldPath: step.New, NewPath: step.Old, Source: outcome.Target})
	}
	if outcome.Verified {
		r.entry.Undone++
		r.entry.Organization.Active = nil
	} else if outcome.Attempted {
		r.result.RequiresReconciliation = true
		r.entry.Organization.Active.MoveOutcome = &outcome
	} else {
		r.entry.Organization.Active = nil
	}
	if err != nil {
		r.entry.UndoFailed = &history.Failure{Operation: step, Error: err.Error()}
		if r.entry.Organization.Active != nil {
			r.entry.Organization.Active.Error = err.Error()
		}
	}
	return errors.Join(err, r.checkpoint())
}

func (r *organizationRun) cleanupDirectories(ctx context.Context) error {
	org := r.entry.Organization
	for org.Cleaned < len(org.Directories) {
		if err := ctx.Err(); err != nil {
			return err
		}
		owned := org.Directories[len(org.Directories)-org.Cleaned-1]
		if r.service.protectsHistory(filepath.Join(owned.Root, filepath.FromSlash(owned.Relative))) {
			return errors.New("protected directory cleanup")
		}
		if err := r.removeOwnedDirectory(ctx, owned); err != nil {
			return err
		}
	}
	return nil
}

func (r *organizationRun) retainDirectory(owned fs.OwnedDirectory, err error) error {
	retained := history.DirectoryRetention{Directory: owned, Reason: err.Error()}
	r.entry.Organization.Retained = append(r.entry.Organization.Retained, retained)
	r.result.DirectoriesRetained = append(r.result.DirectoriesRetained, retained)
	r.entry.Organization.Cleaned++
	r.entry.Organization.Active = nil
	return r.checkpoint()
}

func (r *organizationRun) removeOwnedDirectory(ctx context.Context, owned fs.OwnedDirectory) error {
	directory, err := fs.PrepareDirectoryRemoval(ctx, owned)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return r.retainDirectory(owned, err)
	}
	defer directory.Close()
	req := directory.Request()
	r.entry.Organization.Active = &history.OrganizationStep{Action: "rmdir", Directory: &req}
	if err := r.checkpoint(); err != nil {
		return err
	}
	outcome, err := directory.Execute(ctx)
	r.result.DirectoryOutcomes = append(r.result.DirectoryOutcomes, outcome)
	if outcome.Verified {
		r.result.DirectoriesRemoved = append(r.result.DirectoriesRemoved, owned)
		r.entry.Organization.Cleaned++
		r.entry.Organization.Active = nil
	} else if errors.Is(err, fs.ErrDirectoryNotEmpty) && !outcome.Completed {
		return r.retainDirectory(owned, err)
	} else if outcome.Attempted {
		r.result.RequiresReconciliation = true
		r.entry.Organization.Active.DirectoryOutcome = &outcome
	} else {
		r.entry.Organization.Active = nil
	}
	if err != nil && r.entry.Organization.Active != nil {
		r.entry.Organization.Active.Error = err.Error()
	}
	return errors.Join(err, r.checkpoint())
}

func (r *organizationRun) finishUndo(err error) (Result, error) {
	if err != nil {
		r.entry.Organization.Error = err.Error()
	}
	if r.result.RequiresReconciliation {
		return r.result, err
	}
	r.entry.State = history.PartialUndo
	if err == nil {
		r.entry.State = history.OrganizationUndone
	}
	if checkpointErr := r.checkpoint(); checkpointErr != nil {
		return r.result, errors.Join(err, fmt.Errorf("finalize organization undo: %w", checkpointErr))
	}
	return r.result, err
}
