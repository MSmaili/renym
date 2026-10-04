package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type FileSystemAdapter interface {
	IsValidName(name string) bool
	SanitizeName(name string) string
	IsCaseSensitive() bool
	PathIdentifier(path string) (string, error)
}

type RenameOp struct {
	ID      int
	OldPath string
	NewPath string
	Source  *Snapshot
}

// Snapshot is the identity/version expected at execution time. Directory
// timestamps are not compared: completed child renames legitimately change them.
type Snapshot struct {
	Identity string      `json:"identity"`
	Size     int64       `json:"size"`
	Modified time.Time   `json:"modified"`
	Mode     os.FileMode `json:"mode"`
}

var ErrStalePlan = errors.New("source changed since planning")
var ErrNoReplaceUnsupported = errors.New("safe no-replace rename is unsupported")

type Failure struct {
	Operation RenameOp
	Err       error
}

type Result struct {
	Completed   []RenameOp
	Failed      *Failure
	Unattempted []RenameOp
}

func ValidateName(name string) error {
	length := len(name)
	if runtime.GOOS == "windows" {
		length = len(utf16.Encode([]rune(name)))
	}
	if !utf8.ValidString(name) || length > 255 || strings.ContainsAny(name, `/\`) || !NewAdapter().IsValidName(name) {
		return fmt.Errorf("invalid final basename %q", name)
	}
	return nil
}

func Capture(path string) (*Snapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() && !info.IsDir() {
		return nil, fmt.Errorf("unsupported source kind: %s", path)
	}
	id, err := NewAdapter().PathIdentifier(path)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Identity: id, Size: info.Size(), Modified: info.ModTime(), Mode: info.Mode()}, nil
}

func Revalidate(path string, expected *Snapshot) error {
	actual, err := Capture(path)
	if err != nil {
		return err
	}
	if expected == nil || actual.Identity != expected.Identity || actual.Mode != expected.Mode ||
		(!actual.Mode.IsDir() && (actual.Size != expected.Size || !actual.Modified.Equal(expected.Modified))) {
		return fmt.Errorf("%w: %s", ErrStalePlan, path)
	}
	return nil
}

func targetAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("target already exists: %s: %w", path, os.ErrExist)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func checkPaths(op RenameOp) error {
	if op.OldPath == "" || op.NewPath == "" || filepath.Clean(op.OldPath) != op.OldPath || filepath.Clean(op.NewPath) != op.NewPath || filepath.Dir(op.OldPath) != filepath.Dir(op.NewPath) || op.OldPath == op.NewPath {
		return fmt.Errorf("operation is not an in-place basename rename: %s -> %s", op.OldPath, op.NewPath)
	}
	if err := ValidateName(filepath.Base(op.OldPath)); err != nil {
		return err
	}
	return ValidateName(filepath.Base(op.NewPath))
}

// CheckRename revalidates a single physical step without mutating anything.
func CheckRename(op RenameOp) error {
	if err := checkPaths(op); err != nil {
		return err
	}
	if err := targetAbsent(op.NewPath); err != nil {
		return err
	}
	if op.Source == nil {
		_, err := Capture(op.OldPath)
		return err
	}
	return Revalidate(op.OldPath, op.Source)
}

// Execute applies one batch sequentially and stops at the first error. It never prints or
// overwrites. afterStep checkpoints each completed physical step; callback
// failure stops further mutation and leaves the current step in Completed.
// Paths are assumed to be in trusted local directories, not hostile namespaces.
func Execute(ctx context.Context, ops []RenameOp, afterStep func(RenameOp) error) (Result, error) {
	ops = append([]RenameOp(nil), ops...)
	failed := func(i int, err error, completed []RenameOp) (Result, error) {
		remaining := append([]RenameOp(nil), ops[len(completed):i]...)
		remaining = append(remaining, ops[i+1:]...)
		return Result{Completed: completed, Failed: &Failure{Operation: ops[i], Err: err}, Unattempted: remaining}, err
	}
	seenSources, seenTargets := map[string]bool{}, map[string]bool{}
	// Reject initially occupied destinations, including swaps/chains, before any
	// step. Actual no-replace syscalls also close destination-appearance races.
	for i := range ops {
		if err := ctx.Err(); err != nil {
			return failed(i, err, nil)
		}
		op := &ops[i]
		if err := checkPaths(*op); err != nil {
			return failed(i, err, nil)
		}
		sourceKey, err := filepath.Abs(op.OldPath)
		if err != nil {
			return failed(i, err, nil)
		}
		targetKey, err := filepath.Abs(op.NewPath)
		if err != nil {
			return failed(i, err, nil)
		}
		// Conservative duplicate target comparison needs no filesystem probe.
		targetKey = strings.ToLower(targetKey)
		if seenSources[sourceKey] || seenTargets[targetKey] {
			return failed(i, errors.New("duplicate source or target in batch"), nil)
		}
		seenSources[sourceKey], seenTargets[targetKey] = true, true
		if err := targetAbsent(op.NewPath); err != nil {
			return failed(i, err, nil)
		}
		if op.Source == nil {
			op.Source, err = Capture(op.OldPath)
			if err != nil {
				return failed(i, err, nil)
			}
		}
		if err := CheckRename(*op); err != nil {
			return failed(i, err, nil)
		}
	}
	result := Result{}
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return failed(i, err, result.Completed)
		}
		if err := CheckRename(op); err != nil {
			return failed(i, err, result.Completed)
		}
		if err := renameNoReplace(op.OldPath, op.NewPath); err != nil {
			return failed(i, fmt.Errorf("rename %s to %s: %w", op.OldPath, op.NewPath, err), result.Completed)
		}
		result.Completed = append(result.Completed, op)
		if afterStep != nil {
			if err := afterStep(op); err != nil {
				result.Unattempted = append([]RenameOp(nil), ops[i+1:]...)
				return result, fmt.Errorf("checkpoint failed after rename; history requires reconciliation: %w", err)
			}
		}
	}
	return result, nil
}

func Apply(ops []RenameOp, dryRun bool) error {
	if dryRun {
		return nil
	}
	_, err := Execute(context.Background(), ops, nil)
	return err
}
