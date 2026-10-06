package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrUnsafeMovePath = errors.New("unsafe move path or directory")
var ErrCrossFilesystem = errors.New("cross-filesystem move unsupported")
var ErrMoveAttempted = errors.New("move has already been attempted")
var ErrHardLinkedSource = errors.New("hard-linked move source unsupported on this platform")

const (
	maxMovePathBytes      = 4096
	maxMoveComponents     = 64
	maxMoveDirectoryDepth = 128
)

// MoveRequest uses canonical roots and slash-separated relative names.
type MoveRequest struct {
	SourceRoot           string    `json:"source_root"`
	DestinationRoot      string    `json:"destination_root"`
	SourceDirectory      *Snapshot `json:"source_directory"`
	DestinationDirectory *Snapshot `json:"destination_directory"`
	OldRelative          string    `json:"old_relative"`
	NewRelative          string    `json:"new_relative"`
	Source               *Snapshot `json:"source"`
	SourceParent         *Snapshot `json:"source_parent,omitempty"`
	DestinationParent    *Snapshot `json:"destination_parent,omitempty"`
}

// MoveOutcome requires reconciliation when Completed is true but Verified is false.
type MoveOutcome struct {
	Attempted bool      `json:"attempted"`
	Completed bool      `json:"completed"`
	Verified  bool      `json:"verified"`
	Target    *Snapshot `json:"target,omitempty"`
}

type directoryLink struct {
	file     *os.File
	name     string
	snapshot Snapshot
}

type directoryChain struct {
	links     []directoryLink
	rootIndex int
}

func (c *directoryChain) parent() *os.File { return c.links[len(c.links)-1].file }
func (c *directoryChain) volume() string {
	return identityVolume(c.links[c.rootIndex].snapshot.Identity)
}
func identityVolume(identity string) string {
	volume, _, _ := strings.Cut(identity, ":")
	return volume
}

func (c *directoryChain) close() error {
	var err error
	for i := len(c.links) - 1; i >= 0; i-- {
		err = errors.Join(err, c.links[i].file.Close())
	}
	c.links = nil
	return err
}

// PreparedMove is one-shot, must not be copied, and owns its handles until Close.
// It assumes trusted local directories and one writer, not hostile namespaces.
type PreparedMove struct {
	mu               sync.Mutex
	from, to         *directoryChain
	source           *os.File
	expected         Snapshot
	oldName, newName string
	closed, consumed bool
}

func moveComponents(relative string) ([]string, error) {
	if relative == "" || len(relative) > maxMovePathBytes || strings.Contains(relative, `\`) {
		return nil, ErrUnsafeMovePath
	}
	parts := strings.Split(relative, "/")
	if len(parts) > maxMoveComponents {
		return nil, ErrUnsafeMovePath
	}
	for _, part := range parts {
		if err := ValidateName(part); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnsafeMovePath, err)
		}
	}
	return parts, nil
}

func snapshotHandle(file *os.File) (*Snapshot, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, ErrUnsafeMovePath
	}
	id, err := moveHandleIdentity(file, info)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Identity: id, Mode: info.Mode(), Size: info.Size(), Modified: info.ModTime()}, nil
}

func sameSnapshot(actual, expected *Snapshot) bool {
	return actual != nil && expected != nil && actual.Identity == expected.Identity && actual.Mode == expected.Mode &&
		(actual.Mode.IsDir() || actual.Size == expected.Size && actual.Modified.Equal(expected.Modified))
}

func (c *directoryChain) append(file *os.File, name string) error {
	snapshot, err := snapshotHandle(file)
	if err == nil && !snapshot.Mode.IsDir() {
		err = ErrUnsafeMovePath
	}
	if err != nil {
		_ = file.Close()
		return err
	}
	c.links = append(c.links, directoryLink{file: file, name: name, snapshot: *snapshot})
	return nil
}

func pinMoveParent(ctx context.Context, root string, expected *Snapshot, relative []string) (chain *directoryChain, err error) {
	if root == "" || len(root) > maxMovePathBytes || !filepath.IsAbs(root) || filepath.Clean(root) != root || expected == nil || expected.Identity == "" || !expected.Mode.IsDir() {
		return nil, ErrUnsafeMovePath
	}
	volume, parts, err := moveAbsoluteParts(root)
	if err != nil {
		return nil, err
	}
	if len(parts)+len(relative) > maxMoveDirectoryDepth {
		return nil, ErrUnsafeMovePath
	}
	chain = &directoryChain{links: make([]directoryLink, 0, len(parts)+len(relative)+1)}
	defer func() {
		if err != nil {
			_ = chain.close()
			chain = nil
		}
	}()
	file, err := moveOpenVolume(volume)
	if err != nil {
		return chain, err
	}
	if err = chain.append(file, volume); err != nil {
		return chain, err
	}
	if err = chain.extend(ctx, parts, false); err != nil {
		return chain, err
	}
	chain.rootIndex = len(chain.links) - 1
	if !sameSnapshot(&chain.links[chain.rootIndex].snapshot, expected) {
		return chain, ErrStalePlan
	}
	if err = moveCheckLocal(chain.parent()); err != nil {
		return chain, err
	}
	if err = chain.extend(ctx, relative, true); err != nil {
		return chain, err
	}
	return chain, nil
}

func (c *directoryChain) extend(ctx context.Context, parts []string, beneathRoot bool) error {
	for _, part := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := moveOpenDirectoryAt(c.parent(), part)
		if err != nil {
			return err
		}
		if err := c.append(file, part); err != nil {
			return err
		}
		if !beneathRoot {
			continue
		}
		if identityVolume(c.links[len(c.links)-1].snapshot.Identity) != c.volume() {
			return ErrCrossFilesystem
		}
		if err := moveCheckLocal(c.parent()); err != nil {
			return err
		}
	}
	return nil
}

// PrepareMove validates without mutation. The caller must Close the result.
func PrepareMove(ctx context.Context, req MoveRequest) (*PreparedMove, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	oldParts, err := moveComponents(req.OldRelative)
	if err != nil {
		return nil, err
	}
	newParts, err := moveComponents(req.NewRelative)
	if err != nil {
		return nil, err
	}
	if req.Source == nil || req.Source.Identity == "" || !req.Source.Mode.IsRegular() {
		return nil, ErrUnsafeMovePath
	}
	m := &PreparedMove{expected: *req.Source, oldName: oldParts[len(oldParts)-1], newName: newParts[len(newParts)-1]}
	m.from, err = pinMoveParent(ctx, req.SourceRoot, req.SourceDirectory, oldParts[:len(oldParts)-1])
	if err == nil {
		m.to, err = pinMoveParent(ctx, req.DestinationRoot, req.DestinationDirectory, newParts[:len(newParts)-1])
	}
	if err == nil && m.from.volume() != m.to.volume() {
		err = ErrCrossFilesystem
	}
	if err == nil && (!m.from.matchesParent(req.SourceParent) || !m.to.matchesParent(req.DestinationParent)) {
		err = ErrStalePlan
	}
	if err == nil {
		m.source, err = moveOpenFileAt(m.from.parent(), m.oldName, true)
	}
	if err == nil {
		err = m.check(ctx)
	}
	if err != nil {
		_ = m.Close()
		return nil, err
	}
	return m, nil
}

func (c *directoryChain) check(ctx context.Context) error {
	for i, link := range c.links {
		if err := ctx.Err(); err != nil {
			return err
		}
		var file *os.File
		var err error
		if i == 0 {
			file, err = moveOpenVolume(link.name)
		} else {
			file, err = moveOpenDirectoryAt(c.links[i-1].file, link.name)
		}
		if err != nil {
			return fmt.Errorf("directory lineage changed: %w", err)
		}
		snapshot, err := snapshotHandle(file)
		_ = file.Close()
		if err != nil {
			return err
		}
		if !sameSnapshot(snapshot, &link.snapshot) {
			return ErrStalePlan
		}
	}
	return nil
}

func (c *directoryChain) matchesParent(expected *Snapshot) bool {
	return expected == nil || sameSnapshot(&c.links[len(c.links)-1].snapshot, expected)
}

func (m *PreparedMove) check(ctx context.Context) error {
	if m.closed {
		return os.ErrClosed
	}
	if m.consumed {
		return ErrMoveAttempted
	}
	if m.from == nil || m.to == nil || m.source == nil {
		return os.ErrInvalid
	}
	if err := m.from.check(ctx); err != nil {
		return err
	}
	if err := m.to.check(ctx); err != nil {
		return err
	}
	if err := m.checkSource(); err != nil {
		return err
	}
	return moveEntryAbsent(m.to.parent(), m.newName)
}

func (m *PreparedMove) checkSource() error {
	file, err := moveOpenFileAt(m.from.parent(), m.oldName, false)
	if err != nil {
		return err
	}
	actual, err := snapshotHandle(file)
	_ = file.Close()
	if err != nil {
		return err
	}
	held, err := snapshotHandle(m.source)
	if err != nil {
		return err
	}
	if !sameSnapshot(actual, &m.expected) || !sameSnapshot(held, &m.expected) {
		return ErrStalePlan
	}
	if identityVolume(actual.Identity) != m.from.volume() {
		return ErrCrossFilesystem
	}
	if err := moveCheckSource(m.source); err != nil {
		return err
	}
	return nil
}

// CheckMoveSource validates the source without requiring existing output parents.
func CheckMoveSource(ctx context.Context, req MoveRequest) error {
	parts, err := moveComponents(req.OldRelative)
	if err != nil {
		return err
	}
	if req.Source == nil || req.Source.Identity == "" || !req.Source.Mode.IsRegular() {
		return ErrUnsafeMovePath
	}
	m := &PreparedMove{expected: *req.Source, oldName: parts[len(parts)-1]}
	defer m.Close()
	m.from, err = pinMoveParent(ctx, req.SourceRoot, req.SourceDirectory, parts[:len(parts)-1])
	if err != nil {
		return err
	}
	if !m.from.matchesParent(req.SourceParent) {
		return ErrStalePlan
	}
	m.source, err = moveOpenFileAt(m.from.parent(), m.oldName, true)
	if err != nil {
		return err
	}
	if err := m.from.check(ctx); err != nil {
		return err
	}
	return m.checkSource()
}

func (m *PreparedMove) Check(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.check(ctx)
}

func (m *PreparedMove) Execute(ctx context.Context) (MoveOutcome, error) {
	return m.execute(ctx, func() error {
		return moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName)
	})
}

func (m *PreparedMove) execute(ctx context.Context, rename func() error) (MoveOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.check(ctx); err != nil {
		m.consumed = true
		return MoveOutcome{}, err
	}
	m.consumed = true
	if err := ctx.Err(); err != nil {
		return MoveOutcome{}, err
	}
	result := MoveOutcome{Attempted: true}
	if err := rename(); err != nil {
		return result, err
	}
	result.Completed = true
	// Cancellation cannot hide a completed mutation.
	verificationContext := context.WithoutCancel(ctx)
	if err := errors.Join(m.from.check(verificationContext), m.to.check(verificationContext)); err != nil {
		return result, fmt.Errorf("move completed but directory lineage requires reconciliation: %w", err)
	}
	if err := moveEntryAbsent(m.from.parent(), m.oldName); err != nil {
		return result, fmt.Errorf("rename reported success but source name requires reconciliation: %w", err)
	}
	file, err := moveOpenFileAt(m.to.parent(), m.newName, false)
	if err != nil {
		return result, fmt.Errorf("move completed but target requires reconciliation: %w", err)
	}
	defer file.Close()
	target, err := snapshotHandle(file)
	if err != nil || !sameSnapshot(target, &m.expected) {
		return result, fmt.Errorf("move completed but target requires reconciliation: %w", errors.Join(ErrStalePlan, err))
	}
	result.Verified, result.Target = true, target
	return result, nil
}

func (m *PreparedMove) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var err error
	if m.source != nil {
		err = errors.Join(err, m.source.Close())
	}
	if m.to != nil {
		err = errors.Join(err, m.to.close())
	}
	if m.from != nil {
		err = errors.Join(err, m.from.close())
	}
	m.source, m.from, m.to = nil, nil, nil
	return err
}
