package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/walker"
)

type Service struct {
	adapter fs.FileSystemAdapter
	store   history.Store
}

func NewService(adapter fs.FileSystemAdapter, store history.Store) *Service {
	if adapter == nil {
		adapter = fs.NewAdapter()
	}
	return &Service{adapter: adapter, store: store}
}

func (s *Service) Plan(ctx context.Context, req Request) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	mode, ok := engine.ModeRegistry[req.Mode]
	if !ok {
		return Plan{}, fmt.Errorf("unknown rename mode %q", req.Mode)
	}
	for _, pattern := range req.Ignore {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return Plan{}, fmt.Errorf("invalid ignore pattern %q: %w", pattern, err)
		}
	}
	path, err := filepath.Abs(req.Path)
	if err != nil {
		return Plan{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Plan{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() && !info.IsDir() {
		return Plan{}, fmt.Errorf("unsupported input kind: %s", path)
	}
	req.Path, req.Ignore = path, append([]string(nil), req.Ignore...)
	root := path
	if !info.IsDir() {
		root = filepath.Dir(path)
	}
	paths, err := walker.WalkContext(ctx, walker.Config{
		Path: path, Recursive: req.Recursive, Directories: req.Directories, Files: req.Files,
		Ignore: req.Ignore, NoDefaultIgnore: req.NoDefaultIgnore,
	})
	if err != nil {
		return Plan{}, err
	}
	e := engine.NewEngine(mode, s.adapter)
	var protected []engine.SkippedFile
	filtered := paths[:0]
	for _, path := range paths {
		if s.protectsHistory(path) {
			protected = append(protected, engine.SkippedFile{Path: path, Reason: "protected history directory"})
			continue
		}
		filtered = append(filtered, path)
	}
	paths = filtered
	if req.Directories {
		paths = e.SortPathsByDepth(paths)
	}
	result := e.Plan(paths)
	result.Skipped = append(result.Skipped, protected...)
	plan := Plan{Result: result, root: root, request: req}
	plan.Result.Operations = nil
	for _, op := range result.Operations {
		if err := ctx.Err(); err != nil {
			return Plan{}, err
		}
		snapshot, err := fs.Capture(op.OldPath)
		if err != nil {
			plan.Result.Skipped = append(plan.Result.Skipped, engine.SkippedFile{Path: op.OldPath, Reason: err.Error()})
			continue
		}
		plan.operations = append(plan.operations, fs.RenameOp{ID: len(plan.operations) + 1, OldPath: op.OldPath, NewPath: op.NewPath, Source: snapshot})
		plan.Result.Operations = append(plan.Result.Operations, op)
	}
	return plan, nil
}

func (s *Service) Rename(ctx context.Context, req Request) (Result, error) {
	plan, err := s.Plan(ctx, req)
	if err != nil {
		return Result{}, err
	}
	return s.Execute(ctx, plan)
}

func (s *Service) Execute(ctx context.Context, plan Plan) (Result, error) {
	result := Result{Plan: plan.Result}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if plan.request.DryRun || len(plan.operations) == 0 {
		return result, nil
	}
	if plan.root == "" {
		return result, errors.New("invalid application plan")
	}
	if plan.request.SkipHistory {
		var err error
		result.Execution, err = fs.Execute(ctx, plan.operations, nil)
		return result, err
	}
	if s.store == nil {
		return result, errors.New("history is required; explicitly select skip-history to proceed without it")
	}
	for _, op := range plan.operations {
		if s.protectsHistory(op.OldPath) || s.protectsHistory(op.NewPath) {
			return result, errors.New("plan would rename the history directory or one of its ancestors")
		}
	}
	latest, err := s.store.Latest(plan.root)
	if err != nil && !errors.Is(err, history.ErrNoHistory) {
		return result, fmt.Errorf("check existing history: %w", err)
	}
	if latest != nil && latest.SchemaVersion != 0 && (latest.SchemaVersion != history.SchemaVersion || latest.State != history.Complete && latest.State != history.Partial) {
		return result, errors.New("existing history requires reconciliation or completion of undo before another rename")
	}
	entry := history.Entry{
		SchemaVersion: history.SchemaVersion, State: history.Pending, Timestamp: time.Now().UTC(),
		Command: plan.request.Command, Version: plan.request.Version, Config: plan.request,
		Intent: historyOps(plan.operations), Skipped: historySkips(plan.Result.Skipped), Collisions: historyCollisions(plan.Result.Collisions),
	}
	// Keep manual run ordering stable even if the wall clock moves backwards.
	if latest != nil && !entry.Timestamp.After(latest.Timestamp) {
		entry.Timestamp = latest.Timestamp.Add(time.Nanosecond)
	}
	id, err := s.store.Save(plan.root, entry)
	if err != nil {
		return result, fmt.Errorf("save required rename intent: %w", err)
	}
	result.HistoryID, entry.ID = id, id
	checkpointFailed := false
	result.Execution, err = fs.Execute(ctx, plan.operations, func(op fs.RenameOp) error {
		step := historyOp(op, len(entry.Operations)+1)
		entry.Operations = append(entry.Operations, step)
		if err := s.store.Update(plan.root, id, entry); err != nil {
			checkpointFailed = true
			return err
		}
		return nil
	})
	if checkpointFailed {
		return result, err
	} // Retain uncertain pending intent.
	if len(result.Execution.Completed) == 0 {
		if deleteErr := s.store.DeleteEntry(plan.root, id); deleteErr != nil {
			return result, errors.Join(err, fmt.Errorf("remove unexecuted intent: %w", deleteErr))
		}
		result.HistoryID = ""
		return result, err
	}
	entry.State = history.Complete
	if err != nil {
		entry.State = history.Partial
	}
	entry.Unattempted = historyOps(result.Execution.Unattempted)
	if failed := result.Execution.Failed; failed != nil {
		entry.Failed = &history.Failure{Operation: historyOp(failed.Operation, len(entry.Operations)+1), Error: failed.Err.Error()}
	}
	if updateErr := s.store.Update(plan.root, id, entry); updateErr != nil {
		return result, errors.Join(err, fmt.Errorf("finalize history; retained intent requires reconciliation: %w", updateErr))
	}
	return result, err
}

func (s *Service) Undo(ctx context.Context, root string, dryRun bool) (Result, error) {
	result := Result{}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s.store == nil {
		return result, errors.New("history is required for undo")
	}
	entry, err := s.store.Latest(root)
	if err != nil {
		return result, err
	}
	if entry == nil {
		return result, errors.New("missing history entry")
	}
	if entry.SchemaVersion != history.SchemaVersion {
		return result, errors.New("legacy or unsupported history contains unverified plans; refusing automatic undo")
	}
	if entry.State != history.Complete && entry.State != history.Partial && entry.State != history.PartialUndo {
		return result, errors.New("history requires reconciliation; refusing to replay uncertain steps")
	}
	if entry.Undone < 0 || entry.Undone > len(entry.Operations) || len(entry.Operations) == 0 {
		return result, errors.New("invalid completed-step history")
	}
	var ops []fs.RenameOp
	for i := len(entry.Operations) - entry.Undone - 1; i >= 0; i-- {
		step := entry.Operations[i]
		if step.Source == nil || step.Source.Identity == "" || !filepath.IsAbs(step.Old) || !filepath.IsAbs(step.New) {
			return result, errors.New("history lacks verified absolute steps/identities")
		}
		op := fs.RenameOp{ID: step.ID, OldPath: step.New, NewPath: step.Old, Source: step.Source}
		ops = append(ops, op)
		result.Plan.Operations = append(result.Plan.Operations, engine.RenameOp{OldPath: op.OldPath, NewPath: op.NewPath})
	}
	result.HistoryID = entry.ID
	if dryRun {
		return result, previewUndo(ctx, ops)
	}
	entry.State = history.Undoing
	entry.UndoFailed = nil
	if err := s.store.Update(root, entry.ID, *entry); err != nil {
		return result, fmt.Errorf("save undo intent: %w", err)
	}
	for i, op := range ops {
		checkpointFailed := false
		// Parent reversal restores the exact physical paths for child reversal.
		// Do not stat/rebase every child before its parent has been restored.
		stepResult, stepErr := fs.Execute(ctx, []fs.RenameOp{op}, func(_ fs.RenameOp) error {
			entry.Undone++
			if err := s.store.Update(root, entry.ID, *entry); err != nil {
				checkpointFailed = true
				return err
			}
			return nil
		})
		result.Execution.Completed = append(result.Execution.Completed, stepResult.Completed...)
		if stepErr != nil {
			result.Execution.Failed = stepResult.Failed
			result.Execution.Unattempted = append([]fs.RenameOp(nil), ops[i+1:]...)
			if checkpointFailed {
				return result, stepErr
			}
			entry.State = history.PartialUndo
			if failed := stepResult.Failed; failed != nil {
				entry.UndoFailed = &history.Failure{Operation: historyOp(failed.Operation, len(entry.Operations)-entry.Undone), Error: failed.Err.Error()}
			}
			return result, errors.Join(stepErr, s.store.Update(root, entry.ID, *entry))
		}
	}
	if err := s.store.DeleteEntry(root, entry.ID); err != nil {
		return result, fmt.Errorf("undo completed but history cleanup failed: %w", err)
	}
	result.HistoryID = ""
	return result, nil
}

// Preview evaluates the reversed physical steps against today's tree without
// renaming it. Inverting already-previewed directory renames locates children
// that will become reachable only after their parents are restored.
func previewUndo(ctx context.Context, ops []fs.RenameOp) error {
	var directories []fs.RenameOp
	actualPath := func(path string) string {
		for i := len(directories) - 1; i >= 0; i-- {
			step := directories[i]
			rel, err := filepath.Rel(step.NewPath, path)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				path = filepath.Join(step.OldPath, rel)
			}
		}
		return path
	}
	for _, op := range ops {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := op
		current.OldPath, current.NewPath = actualPath(op.OldPath), actualPath(op.NewPath)
		if err := fs.CheckRename(current); err != nil {
			return err
		}
		if op.Source.Mode.IsDir() {
			directories = append(directories, op)
		}
	}
	return nil
}

func (s *Service) protectsHistory(path string) bool {
	if s.store == nil || s.store.Directory() == "" {
		return false
	}
	state, err := filepath.Abs(s.store.Directory())
	if err != nil {
		return true
	}
	intersects := func(a, b string) bool {
		rel, err := filepath.Rel(a, b)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if intersects(path, state) || intersects(state, path) {
		return true
	}
	// Also cover ordinary existing path aliases. This is not a substitute for
	// rooted traversal against a hostile process changing symlinks concurrently.
	actualState, stateErr := resolveFuturePath(state)
	actualPath, pathErr := filepath.EvalSymlinks(path)
	return stateErr == nil && pathErr == nil && (intersects(actualPath, actualState) || intersects(actualState, actualPath))
}

// Resolve ordinary aliases even when the state leaf has not been created yet.
func resolveFuturePath(path string) (string, error) {
	ancestor := path
	for {
		actual, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			rel, err := filepath.Rel(ancestor, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(actual, rel), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		ancestor = parent
	}
}

func historyOp(op fs.RenameOp, id int) history.Operation {
	if op.ID != 0 {
		id = op.ID
	}
	return history.Operation{Old: op.OldPath, New: op.NewPath, Source: op.Source, ID: id}
}
func historyOps(ops []fs.RenameOp) []history.Operation {
	result := make([]history.Operation, 0, len(ops))
	for i, op := range ops {
		result = append(result, historyOp(op, i+1))
	}
	return result
}
func historySkips(skips []engine.SkippedFile) []history.Skipped {
	result := make([]history.Skipped, 0, len(skips))
	for _, skip := range skips {
		result = append(result, history.Skipped{Path: skip.Path, Reason: skip.Reason})
	}
	return result
}
func historyCollisions(collisions []engine.Collision) []history.Collision {
	result := make([]history.Collision, 0, len(collisions))
	for _, collision := range collisions {
		result = append(result, history.Collision{Source1: collision.Source1, Source2: collision.Source2, Target: collision.Target})
	}
	return result
}
