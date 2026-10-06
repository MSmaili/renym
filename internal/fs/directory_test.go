//go:build linux || darwin || windows

package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func directoryFixture(t *testing.T) DirectoryRequest {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	return DirectoryRequest{Root: root, RootSnapshot: snapshot, Relative: "created"}
}

func createTestDirectory(t *testing.T, req DirectoryRequest) OwnedDirectory {
	t.Helper()
	directory, err := PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	outcome, err := directory.Execute(context.Background())
	if err != nil || !outcome.Completed || !outcome.Verified || outcome.Directory == nil {
		t.Fatalf("create: %+v %v", outcome, err)
	}
	return *outcome.Directory
}

func TestDirectoryPreparationAndOwnedRemoval(t *testing.T) {
	req := directoryFixture(t)
	directory, err := PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if _, err := os.Lstat(filepath.Join(req.Root, req.Relative)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation created directory")
	}
	if directory.Request().ParentSnapshot == nil {
		t.Fatal("parent identity not frozen")
	}
	req.RootSnapshot.Identity = "edited"
	outcome, err := directory.Execute(context.Background())
	if err != nil || !outcome.Verified {
		t.Fatalf("frozen creation: %+v %v", outcome, err)
	}
	if _, err := directory.Execute(context.Background()); !errors.Is(err, ErrMoveAttempted) {
		t.Fatal("creation retried")
	}
	if err := directory.Close(); err != nil {
		t.Fatal(err)
	}
	removal, err := PrepareDirectoryRemoval(context.Background(), *outcome.Directory)
	if err != nil {
		t.Fatal(err)
	}
	defer removal.Close()
	removed, err := removal.Execute(context.Background())
	if err != nil || !removed.Completed || !removed.Verified {
		t.Fatalf("remove: %+v %v", removed, err)
	}
	if _, err := os.Lstat(filepath.Join(req.Root, req.Relative)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("directory retained: %v", err)
	}
}

func TestOwnedRemovalNeverDeletesPopulatedOrReplacedDirectories(t *testing.T) {
	for _, change := range []string{"populated", "replaced", "symlink"} {
		t.Run(change, func(t *testing.T) {
			req := directoryFixture(t)
			owned := createTestDirectory(t, req)
			path := filepath.Join(req.Root, req.Relative)
			if change == "populated" {
				writeMoveBytes(t, filepath.Join(path, "user.txt"), "user bytes")
			} else {
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if change == "symlink" {
					if err := os.Symlink(path+"-original", path); err != nil {
						t.Skip(err)
					}
				} else if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			removal, err := PrepareDirectoryRemoval(context.Background(), owned)
			if change != "populated" {
				if removal != nil {
					_ = removal.Close()
				}
				if err == nil {
					t.Fatal("replacement accepted as owned")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer removal.Close()
			outcome, err := removal.Execute(context.Background())
			if !errors.Is(err, ErrDirectoryNotEmpty) || outcome.Completed {
				t.Fatalf("populated removal: %+v %v", outcome, err)
			}
			assertMoveBytes(t, filepath.Join(path, "user.txt"), "user bytes")
		})
	}
}

func TestDirectoryCreationNativeRacePreservesExistingDirectory(t *testing.T) {
	req := directoryFixture(t)
	directory, err := PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	result, err := directory.execute(context.Background(), func() (bool, error) {
		path := filepath.Join(req.Root, req.Relative)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		writeMoveBytes(t, filepath.Join(path, "user.txt"), "competitor")
		file, completed, err := directoryCreateAt(directory.chain.parent(), directory.name)
		if file != nil {
			_ = file.Close()
		}
		return completed, err
	})
	if err == nil || result.Completed || result.Directory != nil {
		t.Fatalf("exclusive creation: %+v %v", result, err)
	}
	assertMoveBytes(t, filepath.Join(req.Root, req.Relative, "user.txt"), "competitor")
}

func TestDirectoryMutationReportsUncertaintyAndCancellation(t *testing.T) {
	req := directoryFixture(t)
	directory, err := PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	result, err := directory.execute(context.Background(), func() (bool, error) {
		file, completed, err := directoryCreateAt(directory.chain.parent(), directory.name)
		if file != nil {
			_ = file.Close()
		}
		if err != nil {
			return completed, err
		}
		return true, errors.New("identity capture failed")
	})
	if err == nil || !result.Completed || result.Verified || result.Directory != nil {
		t.Fatalf("uncertainty lost: %+v %v", result, err)
	}
	req = directoryFixture(t)
	directory, err = PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result, err = directory.execute(ctx, func() (bool, error) {
		file, completed, err := directoryCreateAt(directory.chain.parent(), directory.name)
		directory.file = file
		cancel()
		return completed, err
	})
	if err != nil || !result.Verified {
		t.Fatalf("cancel hid creation: %+v %v", result, err)
	}
}

func TestDirectoryOperationsRejectTraversalLinksMissingParentsAndChangedRoot(t *testing.T) {
	for _, relative := range []string{"", ".", "..", "../escape", "a//b", `a\b`, "/escape", "missing/child"} {
		req := directoryFixture(t)
		req.Relative = relative
		if directory, err := PrepareDirectoryCreation(context.Background(), req); err == nil {
			_ = directory.Close()
			t.Fatalf("invalid relative: %q", relative)
		}
	}
	req := directoryFixture(t)
	if err := os.Rename(req.Root, req.Root+"-old"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(req.Root); _ = os.Rename(req.Root+"-old", req.Root) })
	if err := os.Mkdir(req.Root, 0700); err != nil {
		t.Fatal(err)
	}
	if directory, err := PrepareDirectoryCreation(context.Background(), req); !errors.Is(err, ErrStalePlan) {
		if directory != nil {
			_ = directory.Close()
		}
		t.Fatalf("changed root: %v", err)
	}
}

func TestDirectoryLineageChangesAfterPreparation(t *testing.T) {
	req := directoryFixture(t)
	if err := os.Mkdir(filepath.Join(req.Root, "parent"), 0700); err != nil {
		t.Fatal(err)
	}
	req.Relative = "parent/child"
	directory, err := PrepareDirectoryCreation(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	parent := filepath.Join(req.Root, "parent")
	if err := os.Rename(parent, parent+"-old"); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		return
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	result, err := directory.Execute(context.Background())
	if err == nil || result.Completed {
		t.Fatalf("changed parent: %+v %v", result, err)
	}
}

func TestPreparedDirectoryConcurrentCheckExecuteClose(t *testing.T) {
	req := directoryFixture(t)
	for i := 0; i < 32; i++ {
		directory, err := PrepareDirectoryCreation(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var wg sync.WaitGroup
		for _, operation := range []func(){func() { _ = directory.Check(context.Background()) }, func() { _, _ = directory.Execute(ctx) }, func() { _ = directory.Close() }} {
			wg.Add(1)
			go func() { defer wg.Done(); operation() }()
		}
		wg.Wait()
		if err := directory.Check(context.Background()); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("close failed to serialize: %v", err)
		}
	}
	if _, err := os.Lstat(filepath.Join(req.Root, req.Relative)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled concurrent creation mutated tree")
	}
}
