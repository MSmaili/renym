package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/templates"
)

type organizationRun struct {
	service  *Service
	root     string
	entry    history.Entry
	result   Result
	bindings map[string]history.DirectoryBinding
}

func (s *Service) executeOrganization(ctx context.Context, plan Plan) (Result, error) {
	result := Result{Plan: plan.Result, Organization: true, SourcePath: plan.root}
	if plan.organization == nil {
		return result, errors.New("missing rooted organization plan")
	}
	if plan.request.DryRun {
		return result, nil
	}
	if plan.request.SkipHistory || s.store == nil {
		return result, ErrOrganizationHistoryRequired
	}
	p := plan.organization
	if err := s.checkOrganizationPlan(ctx, plan); err != nil {
		return result, err
	}
	if len(p.steps) == 0 {
		return result, nil
	}
	run, err := s.beginOrganizationRun(plan, result)
	if err != nil {
		return result, err
	}
	return run.applyOrganization(ctx, p)
}

func (s *Service) beginOrganizationRun(plan Plan, result Result) (*organizationRun, error) {
	p := plan.organization
	latest, err := s.store.Latest(plan.root)
	if err != nil && !errors.Is(err, history.ErrNoHistory) {
		return nil, err
	}
	if err := readyForNewRun(latest); err != nil {
		return nil, err
	}
	run := &organizationRun{service: s, root: plan.root, result: result, entry: history.Entry{
		SchemaVersion: history.OrganizationSchemaVersion, State: history.Pending, Timestamp: time.Now().UTC(), Command: plan.request.Command, Version: plan.request.Version,
		Config: struct {
			Request
			TemplateSpec *templates.Spec `json:"template_spec"`
		}{plan.request, plan.templateSpec},
		Intent: append([]history.Operation(nil), p.steps...), Skipped: historySkips(plan.Result.Skipped), Collisions: historyCollisions(plan.Result.Collisions),
		Organization: &history.Organization{SourceRoot: plan.root, SourceSnapshot: p.source, DirectoryIntents: append([]fs.DirectoryRequest(nil), p.directories...), Bindings: append([]history.DirectoryBinding(nil), p.bindings...)},
	}}
	if latest != nil && !run.entry.Timestamp.After(latest.Timestamp) {
		run.entry.Timestamp = latest.Timestamp.Add(time.Nanosecond)
	}
	id, err := s.store.Save(plan.root, run.entry)
	if err != nil {
		return nil, fmt.Errorf("save required organization intent: %w", err)
	}
	run.entry.ID, run.result.HistoryID = id, id
	return run, nil
}

func (run *organizationRun) applyOrganization(ctx context.Context, p *organizationPlan) (Result, error) {
	for _, req := range p.directories {
		if err := run.createDirectory(ctx, req); err != nil {
			run.recordUnattempted(p.steps)
			return run.finish(err)
		}
	}
	for i, step := range p.steps {
		if err := run.moveFile(ctx, step); err != nil {
			run.recordUnattempted(p.steps[i+1:])
			return run.finish(err)
		}
	}
	return run.finish(nil)
}

func (r *organizationRun) recordUnattempted(steps []history.Operation) {
	r.entry.Unattempted = append([]history.Operation(nil), steps...)
	for _, step := range steps {
		r.result.Execution.Unattempted = append(r.result.Execution.Unattempted, renameOperation(step))
	}
}

func organizationPathAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		return os.ErrExist
	}
	return err
}

func (s *Service) checkOrganizationPlan(ctx context.Context, plan Plan) error {
	p := plan.organization
	if err := checkDirectoryBindings(ctx, p.bindings); err != nil {
		return err
	}
	for _, req := range p.directories {
		path := filepath.Join(req.Root, filepath.FromSlash(req.Relative))
		if s.protectsHistory(path) || protectsTemplate(path, plan.TemplatePath) {
			return errors.New("protected directory creation")
		}
		if err := organizationPathAbsent(path); err != nil {
			return err
		}
	}
	for _, step := range p.steps {
		if s.protectsHistory(step.Old) || s.protectsHistory(step.New) || protectsTemplate(step.New, plan.TemplatePath) {
			return errors.New("protected organization path")
		}
		if err := fs.CheckMoveSource(ctx, *step.Move); err != nil {
			return err
		}
		if err := organizationPathAbsent(step.New); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (r *organizationRun) checkpoint() error {
	if err := r.service.store.Update(r.root, r.entry.ID, r.entry); err != nil {
		r.result.RequiresReconciliation = true
		return fmt.Errorf("organization checkpoint requires reconciliation: %w", err)
	}
	return nil
}

func (r *organizationRun) createDirectory(ctx context.Context, req fs.DirectoryRequest) error {
	if err := r.checkBoundParents(ctx, req.Root, filepath.Dir(directoryPath(req))); err != nil {
		return err
	}
	req.ParentSnapshot = cloneValue(r.bindings[filepath.Dir(directoryPath(req))].Snapshot)
	directory, err := fs.PrepareDirectoryCreation(ctx, req)
	if err != nil {
		return err
	}
	defer directory.Close()
	req = directory.Request()
	r.entry.Organization.Active = &history.OrganizationStep{Action: "mkdir", Directory: &req}
	if err := r.checkpoint(); err != nil {
		return err
	}
	outcome, err := directory.Execute(ctx)
	r.result.DirectoryOutcomes = append(r.result.DirectoryOutcomes, outcome)
	if outcome.Verified {
		r.entry.Organization.Directories = append(r.entry.Organization.Directories, *outcome.Directory)
		r.result.DirectoriesCreated = append(r.result.DirectoriesCreated, *outcome.Directory)
		r.entry.Organization.Bindings = append(r.entry.Organization.Bindings, history.DirectoryBinding{Request: outcome.Directory.DirectoryRequest, Snapshot: outcome.Directory.Snapshot})
		r.bindings[directoryPath(outcome.Directory.DirectoryRequest)] = history.DirectoryBinding{Request: outcome.Directory.DirectoryRequest, Snapshot: outcome.Directory.Snapshot}
		r.entry.Organization.Active = nil
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

func renameOperation(step history.Operation) fs.RenameOp {
	return fs.RenameOp{ID: step.ID, OldPath: step.Old, NewPath: step.New, Source: step.Source}
}

func (r *organizationRun) moveFile(ctx context.Context, step history.Operation) error {
	if err := r.checkMoveParents(ctx, *step.Move); err != nil {
		return r.moveFailure(step, err)
	}
	req := *step.Move
	req.SourceParent = cloneValue(r.bindings[filepath.Dir(step.Old)].Snapshot)
	req.DestinationParent = cloneValue(r.bindings[filepath.Dir(step.New)].Snapshot)
	step.Move = &req
	move, err := fs.PrepareMove(ctx, req)
	if err != nil {
		return r.moveFailure(step, err)
	}
	defer move.Close()
	r.entry.Organization.Active = &history.OrganizationStep{Action: "move", OperationID: step.ID, Move: &req}
	if err := r.checkpoint(); err != nil {
		return err
	}
	outcome, err := move.Execute(ctx)
	r.result.MoveOutcomes = append(r.result.MoveOutcomes, outcome)
	if outcome.Completed {
		completed := renameOperation(step)
		completed.Source = outcome.Target
		r.result.Execution.Completed = append(r.result.Execution.Completed, completed)
	}
	if outcome.Verified {
		step.Source = outcome.Target
		r.entry.Operations = append(r.entry.Operations, step)
		r.entry.Organization.Active = nil
	} else if outcome.Attempted {
		r.result.RequiresReconciliation = true
		r.entry.Organization.Active.MoveOutcome = &outcome
	} else {
		r.entry.Organization.Active = nil
	}
	if err != nil {
		r.moveFailure(step, err)
		if r.entry.Organization.Active != nil {
			r.entry.Organization.Active.Error = err.Error()
		}
	}
	return errors.Join(err, r.checkpoint())
}

func (r *organizationRun) moveFailure(step history.Operation, err error) error {
	r.result.Execution.Failed = &fs.Failure{Operation: renameOperation(step), Err: err}
	r.entry.Failed = &history.Failure{Operation: step, Error: err.Error()}
	return err
}

func (r *organizationRun) finish(err error) (Result, error) {
	if err != nil {
		r.entry.Organization.Error = err.Error()
	}
	if r.result.RequiresReconciliation {
		return r.result, err
	}
	r.entry.State = history.Complete
	if err != nil {
		r.entry.State = history.Partial
	}
	if checkpointErr := r.checkpoint(); checkpointErr != nil {
		return r.result, errors.Join(err, checkpointErr)
	}
	if len(r.entry.Operations) == 0 && len(r.entry.Organization.Directories) == 0 {
		if deleteErr := r.service.store.DeleteEntry(r.root, r.entry.ID); deleteErr != nil {
			return r.result, errors.Join(err, deleteErr)
		}
		r.result.HistoryID = ""
	}
	return r.result, err
}
