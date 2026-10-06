package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
)

var ErrDirectoryNotEmpty = errors.New("owned directory is not empty")

type DirectoryRequest struct {
	Root           string    `json:"root"`
	Relative       string    `json:"relative"`
	RootSnapshot   *Snapshot `json:"root_snapshot"`
	ParentSnapshot *Snapshot `json:"parent_snapshot,omitempty"`
}

type OwnedDirectory struct {
	DirectoryRequest
	Snapshot *Snapshot `json:"snapshot"`
}

type DirectoryOutcome struct {
	Attempted bool            `json:"attempted"`
	Completed bool            `json:"completed"`
	Verified  bool            `json:"verified"`
	Directory *OwnedDirectory `json:"directory,omitempty"`
}

// PreparedDirectory is one-shot and must be closed. Unix creation assumes one writer.
type PreparedDirectory struct {
	mu                       sync.Mutex
	chain                    *directoryChain
	file                     *os.File
	request                  DirectoryRequest
	expected                 *Snapshot
	name                     string
	remove, consumed, closed bool
}

func cloneSnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}
	copy := *snapshot
	return &copy
}

func cloneDirectoryRequest(req DirectoryRequest) DirectoryRequest {
	req.RootSnapshot, req.ParentSnapshot = cloneSnapshot(req.RootSnapshot), cloneSnapshot(req.ParentSnapshot)
	return req
}

func InspectDirectory(ctx context.Context, req DirectoryRequest) (*Snapshot, error) {
	var parts []string
	var err error
	if req.Relative != "" {
		parts, err = moveComponents(req.Relative)
	}
	if err != nil {
		return nil, err
	}
	chain, err := pinMoveParent(ctx, req.Root, req.RootSnapshot, parts)
	if err != nil {
		return nil, err
	}
	defer chain.close()
	if err := chain.check(ctx); err != nil {
		return nil, err
	}
	return cloneSnapshot(&chain.links[len(chain.links)-1].snapshot), nil
}

func PrepareDirectoryCreation(ctx context.Context, req DirectoryRequest) (*PreparedDirectory, error) {
	return prepareDirectory(ctx, req, nil)
}

func PrepareDirectoryRemoval(ctx context.Context, owned OwnedDirectory) (*PreparedDirectory, error) {
	if owned.Snapshot == nil || !owned.Snapshot.Mode.IsDir() || owned.Snapshot.Identity == "" || owned.ParentSnapshot == nil {
		return nil, ErrUnsafeMovePath
	}
	return prepareDirectory(ctx, owned.DirectoryRequest, owned.Snapshot)
}

func prepareDirectory(ctx context.Context, req DirectoryRequest, expected *Snapshot) (*PreparedDirectory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parts, err := moveComponents(req.Relative)
	if err != nil {
		return nil, err
	}
	d := &PreparedDirectory{request: cloneDirectoryRequest(req), expected: cloneSnapshot(expected), name: parts[len(parts)-1], remove: expected != nil}
	d.chain, err = pinMoveParent(ctx, req.Root, req.RootSnapshot, parts[:len(parts)-1])
	if err == nil {
		parent := &d.chain.links[len(d.chain.links)-1].snapshot
		if req.ParentSnapshot != nil && !sameSnapshot(parent, req.ParentSnapshot) {
			err = ErrStalePlan
		} else {
			d.request.ParentSnapshot = cloneSnapshot(parent)
		}
	}
	if err == nil && d.remove {
		d.file, err = directoryOpenAt(d.chain.parent(), d.name, true)
	}
	if err == nil {
		err = d.check(ctx)
	}
	if err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

func (d *PreparedDirectory) Request() DirectoryRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return cloneDirectoryRequest(d.request)
}

func (d *PreparedDirectory) check(ctx context.Context) error {
	if d.closed {
		return os.ErrClosed
	}
	if d.consumed {
		return ErrMoveAttempted
	}
	if d.chain == nil {
		return os.ErrInvalid
	}
	if err := d.chain.check(ctx); err != nil {
		return err
	}
	if !d.remove {
		return moveEntryAbsent(d.chain.parent(), d.name)
	}
	held, err := snapshotHandle(d.file)
	if err != nil {
		return err
	}
	if !sameSnapshot(held, d.expected) {
		return ErrStalePlan
	}
	return d.checkName(d.expected)
}

func (d *PreparedDirectory) checkName(expected *Snapshot) error {
	file, err := directoryOpenAt(d.chain.parent(), d.name, false)
	if err != nil {
		return err
	}
	snapshot, err := snapshotHandle(file)
	_ = file.Close()
	if err != nil {
		return err
	}
	if !sameSnapshot(snapshot, expected) {
		return ErrStalePlan
	}
	return nil
}

func (d *PreparedDirectory) Check(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.check(ctx)
}

func (d *PreparedDirectory) Execute(ctx context.Context) (DirectoryOutcome, error) {
	return d.execute(ctx, func() (bool, error) {
		if d.remove {
			err := directoryRemoveAt(d.chain.parent(), d.name, d.file)
			if err != nil {
				return false, err
			}
			// Windows disposition removes the name when this handle closes.
			err = d.file.Close()
			d.file = nil
			return true, err
		}
		file, completed, err := directoryCreateAt(d.chain.parent(), d.name)
		d.file = file
		return completed, err
	})
}

func (d *PreparedDirectory) execute(ctx context.Context, mutate func() (bool, error)) (DirectoryOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.check(ctx); err != nil {
		d.consumed = true
		return DirectoryOutcome{}, err
	}
	d.consumed = true
	if err := ctx.Err(); err != nil {
		return DirectoryOutcome{}, err
	}
	result := DirectoryOutcome{Attempted: true}
	completed, err := mutate()
	result.Completed = completed
	if err != nil {
		return result, err
	}
	if !completed {
		return result, errors.New("directory mutation did not complete")
	}
	if err := d.chain.check(context.WithoutCancel(ctx)); err != nil {
		return result, fmt.Errorf("directory mutation requires reconciliation: %w", err)
	}
	if d.remove {
		if err := directoryRemovalVerified(d.chain.parent(), d.name); err != nil {
			return result, fmt.Errorf("directory removal requires reconciliation: %w", err)
		}
		result.Verified = true
		return result, nil
	}
	snapshot, err := snapshotHandle(d.file)
	if err == nil && (!snapshot.Mode.IsDir() || identityVolume(snapshot.Identity) != d.chain.volume()) {
		err = ErrUnsafeMovePath
	}
	if err == nil {
		err = d.checkName(snapshot)
	}
	if err != nil {
		return result, fmt.Errorf("directory creation requires reconciliation: %w", err)
	}
	result.Verified = true
	result.Directory = &OwnedDirectory{DirectoryRequest: cloneDirectoryRequest(d.request), Snapshot: snapshot}
	return result, nil
}

func (d *PreparedDirectory) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	var err error
	if d.file != nil {
		err = errors.Join(err, d.file.Close())
	}
	if d.chain != nil {
		err = errors.Join(err, d.chain.close())
	}
	d.file, d.chain = nil, nil
	return err
}
