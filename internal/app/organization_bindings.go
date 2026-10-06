package app

import (
	"context"
	"path/filepath"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

func (r *organizationRun) checkBoundParents(ctx context.Context, root, parent string) error {
	if r.bindings == nil {
		r.bindings = make(map[string]history.DirectoryBinding, len(r.entry.Organization.Bindings))
		for _, binding := range r.entry.Organization.Bindings {
			r.bindings[directoryPath(binding.Request)] = binding
		}
	}
	parts, err := parentRelatives(root, parent)
	if err != nil {
		return err
	}
	for _, relative := range parts {
		binding, ok := r.bindings[filepath.Join(root, filepath.FromSlash(relative))]
		if !ok {
			return fs.ErrStalePlan
		}
		if err := checkDirectoryBindings(ctx, []history.DirectoryBinding{binding}); err != nil {
			return err
		}
	}
	return nil
}

func (r *organizationRun) checkMoveParents(ctx context.Context, req fs.MoveRequest) error {
	old, err := absoluteRelative(req.SourceRoot, req.OldRelative)
	if err != nil {
		return err
	}
	new, err := absoluteRelative(req.DestinationRoot, req.NewRelative)
	if err != nil {
		return err
	}
	if err := r.checkBoundParents(ctx, req.SourceRoot, filepath.Dir(old)); err != nil {
		return err
	}
	return r.checkBoundParents(ctx, req.DestinationRoot, filepath.Dir(new))
}
