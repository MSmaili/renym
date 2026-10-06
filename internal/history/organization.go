package history

import (
	"path/filepath"

	"github.com/MSmaili/renym/internal/fs"
)

func directorySnapshot(snapshot *fs.Snapshot) bool {
	return snapshot != nil && snapshot.Identity != "" && snapshot.Mode.IsDir()
}

func organizationUndoComplete(entry *Entry) bool {
	org := entry.Organization
	if entry.SchemaVersion != OrganizationSchemaVersion || entry.State != OrganizationUndone || org == nil {
		return false
	}
	if org.Active != nil || entry.Undone != len(entry.Operations) || org.Cleaned != len(org.Directories) || !directorySnapshot(org.SourceSnapshot) || org.SourceRoot != entry.Path || org.SourceSnapshot.Identity != entry.DirID || len(org.Bindings) == 0 {
		return false
	}
	for _, step := range entry.Operations {
		if !validAuditMove(step) {
			return false
		}
	}
	for _, owned := range org.Directories {
		if !directorySnapshot(owned.Snapshot) || !directorySnapshot(owned.RootSnapshot) || !directorySnapshot(owned.ParentSnapshot) || !validRootedPath(owned.Root, owned.Relative) {
			return false
		}
	}
	for _, binding := range org.Bindings {
		if !directorySnapshot(binding.Snapshot) || !directorySnapshot(binding.Request.RootSnapshot) || !filepath.IsAbs(binding.Request.Root) {
			return false
		}
	}
	return true
}

func validRootedPath(root, relative string) bool {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	local, err := filepath.Localize(relative)
	return err == nil && local != "."
}

func validAuditMove(step Operation) bool {
	req := step.Move
	if req == nil || step.Source == nil || step.Source.Identity == "" || !step.Source.Mode.IsRegular() || !directorySnapshot(req.SourceDirectory) || !directorySnapshot(req.DestinationDirectory) {
		return false
	}
	if !directorySnapshot(req.SourceParent) || !directorySnapshot(req.DestinationParent) {
		return false
	}
	if !validRootedPath(req.SourceRoot, req.OldRelative) || !validRootedPath(req.DestinationRoot, req.NewRelative) {
		return false
	}
	return step.Old == filepath.Join(req.SourceRoot, filepath.FromSlash(req.OldRelative)) && step.New == filepath.Join(req.DestinationRoot, filepath.FromSlash(req.NewRelative)) && step.Old != step.New
}
