package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
)

type organizationPlan struct {
	steps       []history.Operation
	directories []fs.DirectoryRequest
	bindings    []history.DirectoryBinding
	source      *fs.Snapshot
}

type organizationPlanner struct {
	service            *Service
	root, templatePath string
	plan               *organizationPlan
	seenBindings       map[string]bool
	seenDirectories    map[string]string
	sources            map[string]fs.Snapshot
}

func (s *Service) bindOrganizationPlan(ctx context.Context, plan Plan) (Plan, error) {
	p := &organizationPlan{}
	p.source = cloneValue(plan.sourceDirectory)
	planner := organizationPlanner{service: s, root: plan.root, templatePath: plan.TemplatePath, plan: p, seenBindings: map[string]bool{}, seenDirectories: map[string]string{}, sources: plan.proposalSources}
	for i, op := range plan.Result.Operations {
		if err := planner.addStep(ctx, op, i+1); err != nil {
			return Plan{}, err
		}
	}
	sort.SliceStable(p.directories, func(i, j int) bool {
		a, b := directoryPath(p.directories[i]), directoryPath(p.directories[j])
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
	plan.organization = p
	plan.proposalSources = nil
	return plan, nil
}

func directoryPath(req fs.DirectoryRequest) string {
	return filepath.Join(req.Root, filepath.FromSlash(req.Relative))
}

func existingDirectoryAnchor(ctx context.Context, parent string) (string, *fs.Snapshot, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		_, err := os.Lstat(parent)
		if err == nil {
			snapshot, err := fs.Capture(parent)
			return parent, snapshot, err
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(parent) == parent {
			return "", nil, err
		}
		parent = filepath.Dir(parent)
	}
}

func (p *organizationPlanner) addStep(ctx context.Context, op engine.RenameOp, id int) error {
	anchor, anchorSnapshot, err := existingDirectoryAnchor(ctx, filepath.Dir(op.NewPath))
	if err != nil {
		return err
	}
	oldRelative, err := relativeBelow(p.root, op.OldPath)
	if err != nil {
		return err
	}
	newRelative, err := relativeBelow(anchor, op.NewPath)
	if err != nil {
		return err
	}
	frozen, ok := p.sources[op.OldPath]
	if !ok {
		return errors.New("missing frozen organization source")
	}
	source := &frozen
	if volumeIdentity(source.Identity) != volumeIdentity(anchorSnapshot.Identity) {
		return fs.ErrCrossFilesystem
	}
	move := fs.MoveRequest{SourceRoot: p.root, SourceDirectory: p.plan.source, DestinationRoot: anchor, DestinationDirectory: anchorSnapshot, OldRelative: oldRelative, NewRelative: newRelative, Source: source}
	if err := fs.CheckMoveSource(ctx, move); err != nil {
		return err
	}
	p.plan.steps = append(p.plan.steps, history.Operation{ID: id, Old: op.OldPath, New: op.NewPath, Source: source, Move: &move})
	if err := p.bindParents(ctx, p.root, p.plan.source, filepath.Dir(op.OldPath)); err != nil {
		return err
	}
	if err := p.bindParents(ctx, anchor, anchorSnapshot, anchor); err != nil {
		return err
	}
	return p.addDirectories(anchor, anchorSnapshot, filepath.Dir(op.NewPath))
}

func (p *organizationPlanner) addDirectories(anchor string, snapshot *fs.Snapshot, parent string) error {
	if parent == anchor {
		return nil
	}
	rel, err := relativeBelow(anchor, parent)
	if err != nil {
		return err
	}
	current := anchor
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		key := strings.ToLower(current)
		if existing := p.seenDirectories[key]; existing != "" {
			if existing != current {
				return errors.New("case-ambiguous destination directory")
			}
			continue
		}
		if p.service.protectsHistory(current) || protectsTemplate(current, p.templatePath) {
			return errors.New("protected directory creation")
		}
		relative, err := relativeBelow(anchor, current)
		if err != nil {
			return err
		}
		p.plan.directories = append(p.plan.directories, fs.DirectoryRequest{Root: anchor, RootSnapshot: snapshot, Relative: relative})
		p.seenDirectories[key] = current
	}
	return nil
}

func relativeBelow(root, name string) (string, error) {
	rel, err := filepath.Rel(root, name)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", fs.ErrUnsafeMovePath
	}
	return filepath.ToSlash(rel), nil
}

func parentRelatives(root, parent string) ([]string, error) {
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel != "." && !filepath.IsLocal(rel) {
		return nil, fs.ErrUnsafeMovePath
	}
	parts := []string{""}
	if rel == "." {
		return parts, nil
	}
	current := ""
	for _, component := range strings.Split(filepath.ToSlash(rel), "/") {
		if current != "" {
			current += "/"
		}
		current += component
		parts = append(parts, current)
	}
	return parts, nil
}

func (p *organizationPlanner) bindParents(ctx context.Context, root string, rootSnapshot *fs.Snapshot, parent string) error {
	parts, err := parentRelatives(root, parent)
	if err != nil {
		return err
	}
	for _, relative := range parts {
		key := filepath.Join(root, filepath.FromSlash(relative))
		if p.seenBindings[key] {
			continue
		}
		req := fs.DirectoryRequest{Root: root, RootSnapshot: rootSnapshot, Relative: relative}
		snapshot, err := fs.InspectDirectory(ctx, req)
		if err != nil {
			return fmt.Errorf("bind parent: %w", err)
		}
		p.plan.bindings = append(p.plan.bindings, history.DirectoryBinding{Request: req, Snapshot: snapshot})
		p.seenBindings[key] = true
	}
	return nil
}

func checkDirectoryBindings(ctx context.Context, bindings []history.DirectoryBinding) error {
	for _, binding := range bindings {
		actual, err := fs.InspectDirectory(ctx, binding.Request)
		if err != nil {
			return err
		}
		if binding.Snapshot == nil || actual.Identity != binding.Snapshot.Identity || actual.Mode != binding.Snapshot.Mode {
			return fs.ErrStalePlan
		}
	}
	return nil
}
