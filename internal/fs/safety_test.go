package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeRenameNeverReplacesAnExistingDestination(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeBytes(t, source, "source bytes")
	writeBytes(t, target, "destination bytes")
	// Exercise the syscall directly, bypassing Lstat. This is the destination
	// race defense, not just a check-then-overwriting-rename test.
	if err := renameNoReplace(source, target); err == nil {
		t.Fatal("native call overwrote destination")
	}
	assertBytes(t, source, "source bytes")
	assertBytes(t, target, "destination bytes")
}

func TestExecuteReportsCompletedFailedAndUnattempted(t *testing.T) {
	root := t.TempDir()
	var ops []RenameOp
	for _, name := range []string{"a", "b", "c"} {
		old := filepath.Join(root, name)
		writeBytes(t, old, name)
		ops = append(ops, RenameOp{OldPath: old, NewPath: old + "-new"})
	}
	result, err := Execute(context.Background(), ops, func(op RenameOp) error {
		writeBytes(t, ops[1].NewPath, "arrived after planning")
		return nil
	})
	if err == nil || len(result.Completed) != 1 || result.Failed == nil || len(result.Unattempted) != 1 {
		t.Fatalf("wrong outcome: %+v, %v", result, err)
	}
	assertBytes(t, ops[0].NewPath, "a")
	assertBytes(t, ops[1].OldPath, "b")
	assertBytes(t, ops[2].OldPath, "c")
	assertBytes(t, ops[1].NewPath, "arrived after planning")
}

func TestExecuteDetectsReplacedIdentity(t *testing.T) {
	root := t.TempDir()
	source, replacement, target := filepath.Join(root, "source"), filepath.Join(root, "replacement"), filepath.Join(root, "target")
	writeBytes(t, source, "original")
	snapshot, err := Capture(source)
	if err != nil {
		t.Fatal(err)
	}
	writeBytes(t, replacement, "replaced")
	if err := os.Chtimes(replacement, snapshot.Modified, snapshot.Modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, source); err != nil {
		t.Fatal(err)
	}
	_, err = Execute(context.Background(), []RenameOp{{OldPath: source, NewPath: target, Source: snapshot}}, nil)
	if !errors.Is(err, ErrStalePlan) {
		t.Fatalf("replacement identity accepted: %v", err)
	}
	assertBytes(t, source, "replaced")
}

func TestExecuteCancellationAfterCompletedStep(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	writeBytes(t, a, "A")
	writeBytes(t, b, "B")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := Execute(ctx, []RenameOp{{OldPath: a, NewPath: a + "-new"}, {OldPath: b, NewPath: b + "-new"}}, func(RenameOp) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || len(result.Completed) != 1 {
		t.Fatalf("wrong canceled outcome: %+v, %v", result, err)
	}
	assertBytes(t, a+"-new", "A")
	assertBytes(t, b, "B")
}

func TestApplyRejectsOccupiedTargetsWithoutLosingBytes(t *testing.T) {
	for _, name := range []string{"swap", "chain", "occupied"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			a, b, c := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "c")
			writeBytes(t, a, "A")
			writeBytes(t, b, "B")
			ops := []RenameOp{{OldPath: a, NewPath: b}}
			if name == "swap" {
				ops = append(ops, RenameOp{OldPath: b, NewPath: a})
			}
			if name == "chain" {
				ops = append(ops, RenameOp{OldPath: b, NewPath: c})
			}
			if err := Apply(ops, false); err == nil {
				t.Fatal("occupied-target plan was accepted")
			}
			assertBytes(t, a, "A")
			assertBytes(t, b, "B")
			if _, err := os.Lstat(c); !os.IsNotExist(err) {
				t.Fatalf("unexpected chain destination: %v", err)
			}
		})
	}
}

func TestApplyRejectsDanglingDestination(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeBytes(t, source, "source bytes")
	if err := os.Symlink(filepath.Join(root, "missing"), target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := Apply([]RenameOp{{OldPath: source, NewPath: target}}, false); err == nil {
		t.Fatal("dangling destination replaced")
	}
	assertBytes(t, source, "source bytes")
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("destination link lost: %v", err)
	}
}

func writeBytes(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, %v; want %q", path, got, err, want)
	}
}
