package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/templates"
)

var ErrReadOnlyPlan = errors.New("read-only organization plans cannot be converted to apply; request a new non-preview plan")
var ErrOrganizationHistoryRequired = errors.New("organization requires history; --skip-history is unsupported for apply")

type organizationPreview struct{ destinations map[string]destinationPreview }
type destinationPreview struct{ root, anchor, volume string }

func resolveDeclaredRoot(root string) (string, error) {
	if err := templates.ValidateRoot(root); err != nil {
		return "", err
	}
	if strings.HasPrefix(root, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(home) {
			return "", errors.New("home directory must be absolute")
		}
		root = filepath.Join(home, filepath.FromSlash(root[2:]))
	}
	return filepath.Clean(root), nil
}

func volumeIdentity(identity string) string {
	volume, _, _ := strings.Cut(identity, ":")
	return volume
}

// Preparation and checks are read-only preview diagnostics, NOT a mutation
// containment protocol. Root handles/no-replace/journals must precede apply.
func prepareOrganization(source string, compiled *templates.Compiled) (*organizationPreview, error) {
	preview := &organizationPreview{destinations: make(map[string]destinationPreview)}
	for _, rule := range compiled.Snapshot().Rules {
		if rule.Move == nil {
			continue
		}
		literal := rule.Move.Root
		if _, ok := preview.destinations[literal]; ok {
			continue
		}
		root, err := resolveDeclaredRoot(literal)
		if err != nil {
			return nil, fmt.Errorf("rule %q move.root: %w", rule.ID, err)
		}
		if info, err := os.Lstat(root); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return nil, fmt.Errorf("rule %q move.root: expected a directory, not a link/special file", rule.ID)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("rule %q move.root: %w", rule.ID, err)
		}
		root, err = resolveFuturePath(root)
		if err != nil {
			return nil, fmt.Errorf("rule %q move.root: %w", rule.ID, err)
		}
		if pathContains(root, source) || pathContains(source, root) {
			return nil, fmt.Errorf("rule %q move.root: source and destination roots must not overlap", rule.ID)
		}
		anchor := root
		for {
			info, err := os.Lstat(anchor)
			if err == nil {
				if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("rule %q move.root: invalid destination ancestor", rule.ID)
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || filepath.Dir(anchor) == anchor {
				return nil, fmt.Errorf("rule %q move.root: %w", rule.ID, err)
			}
			anchor = filepath.Dir(anchor)
		}
		id, err := fs.NewAdapter().PathIdentifier(anchor)
		if err != nil {
			return nil, fmt.Errorf("rule %q move.root: %w", rule.ID, err)
		}
		preview.destinations[literal] = destinationPreview{root: root, anchor: anchor, volume: volumeIdentity(id)}
	}
	return preview, nil
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(strings.ToLower(parent), strings.ToLower(child))
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

func (d destinationPreview) missingDirectories(parent string) ([]string, error) {
	rel, err := filepath.Rel(d.anchor, parent)
	if err != nil || rel != "." && !filepath.IsLocal(rel) {
		return nil, errors.New("destination directory escapes its declared root")
	}
	var missing []string
	if rel == "." {
		return missing, nil
	}
	current := d.anchor
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, current)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect destination directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("destination directory is a link or non-directory: %s", current)
		}
		id, err := fs.NewAdapter().PathIdentifier(current)
		if err != nil {
			return nil, err
		}
		if volumeIdentity(id) != d.volume {
			return nil, errors.New("destination crosses a filesystem boundary")
		}
	}
	return missing, nil
}
