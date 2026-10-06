//go:build linux || darwin || windows

package fs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func moveFixture(t testing.TB) MoveRequest {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	from, to := filepath.Join(base, "input"), filepath.Join(base, "output")
	for _, root := range []string{from, to} {
		if err := os.MkdirAll(filepath.Join(root, "nested"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	old := filepath.Join(from, "nested", "source.txt")
	writeMoveBytes(t, old, "source bytes")
	capture := func(path string) *Snapshot {
		snapshot, err := Capture(path)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	return MoveRequest{SourceRoot: from, DestinationRoot: to, SourceDirectory: capture(from), DestinationDirectory: capture(to), OldRelative: "nested/source.txt", NewRelative: "nested/target.txt", Source: capture(old)}
}

func prepareTestMove(t *testing.T, req MoveRequest) *PreparedMove {
	t.Helper()
	m, err := PrepareMove(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func moveOld(req MoveRequest) string {
	return filepath.Join(req.SourceRoot, filepath.FromSlash(req.OldRelative))
}
func moveNew(req MoveRequest) string {
	return filepath.Join(req.DestinationRoot, filepath.FromSlash(req.NewRelative))
}

func writeMoveBytes(t testing.TB, path, content string) {
	t.Helper()
	file, err := openMoveTestFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(file, content)
	err = errors.Join(err, file.Close())
	if err != nil {
		t.Fatal(err)
	}
}

func assertMoveBytes(t *testing.T, path, expected string) {
	t.Helper()
	file, err := openMoveTestFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(file)
	err = errors.Join(err, file.Close())
	if err != nil || string(content) != expected {
		t.Fatalf("%s: got %q, want %q: %v", path, content, expected, err)
	}
}

func TestPreparedMoveRoundTripAndFrozenSnapshots(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	if err := m.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if _, err := os.Lstat(moveNew(req)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation mutated destination")
	}
	identity := req.Source.Identity
	req.Source.Identity, req.SourceDirectory.Identity, req.DestinationDirectory.Identity = "changed", "changed", "changed"
	outcome, err := m.Execute(context.Background())
	if err != nil || !outcome.Completed || !outcome.Verified || outcome.Target == nil || outcome.Target.Identity != identity {
		t.Fatalf("move: %+v %v", outcome, err)
	}
	if _, err := os.Lstat(moveOld(req)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source survived successful move: %v", err)
	}
	assertMoveBytes(t, moveNew(req), "source bytes")
	if _, err := m.Execute(context.Background()); !errors.Is(err, ErrMoveAttempted) {
		t.Fatalf("move attempted twice: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	from, err := Capture(req.DestinationRoot)
	if err != nil {
		t.Fatal(err)
	}
	to, err := Capture(req.SourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	inverse := prepareTestMove(t, MoveRequest{SourceRoot: req.DestinationRoot, DestinationRoot: req.SourceRoot, SourceDirectory: from, DestinationDirectory: to, OldRelative: req.NewRelative, NewRelative: req.OldRelative, Source: outcome.Target})
	restored, err := inverse.Execute(context.Background())
	if err != nil || !restored.Verified {
		t.Fatalf("inverse primitive: %+v %v", restored, err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if _, err := os.Lstat(moveNew(req)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("inverse retained target")
	}
}

func TestPreparedMoveNativeDestinationAppearanceRace(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	result, err := m.execute(context.Background(), func() error {
		// Inject the conflict after the final absence check.
		writeMoveBytes(t, moveNew(req), "competitor bytes")
		return moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName)
	})
	if err == nil || !result.Attempted || result.Completed || result.Verified {
		t.Fatalf("native race overwrote: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	assertMoveBytes(t, moveNew(req), "competitor bytes")
	if _, err := m.Execute(context.Background()); !errors.Is(err, ErrMoveAttempted) {
		t.Fatal("native failure was retryable")
	}
}

func TestPreparedMoveNativeHardLinkDestinationAppearanceRace(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	result, err := m.execute(context.Background(), func() error {
		if err := os.Link(moveOld(req), moveNew(req)); err != nil {
			if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
				t.Skip("pinned Windows source denies hard-link race fixture's delete sharing")
			}
			t.Fatal(err)
		}
		return moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName)
	})
	expectNativeSuccess := runtime.GOOS == "windows"
	if !errors.Is(err, os.ErrExist) || !result.Attempted || result.Completed != expectNativeSuccess || result.Verified || result.Target != nil {
		t.Fatalf("native hard-link conflict treated as verified move: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	assertMoveBytes(t, moveNew(req), "source bytes")
}

func TestPreparedMoveRejectsSuccessfulNativeNoOp(t *testing.T) {
	req := moveFixture(t)
	move := prepareTestMove(t, req)
	result, err := move.execute(context.Background(), func() error { return nil })
	if !errors.Is(err, os.ErrExist) || !result.Attempted || !result.Completed || result.Verified || result.Target != nil {
		t.Fatalf("native no-op treated as verified move: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if _, err := os.Lstat(moveNew(req)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no-op changed destination: %v", err)
	}
}

func TestPreparedMoveReoccupiedSourceRequiresReconciliation(t *testing.T) {
	req := moveFixture(t)
	move := prepareTestMove(t, req)
	result, err := move.execute(context.Background(), func() error {
		if err := moveRenameNoReplace(move.from.parent(), move.oldName, move.source, move.to.parent(), move.newName); err != nil {
			return err
		}
		writeMoveBytes(t, moveOld(req), "new arrival")
		return nil
	})
	if !errors.Is(err, os.ErrExist) || !result.Completed || result.Verified || result.Target != nil {
		t.Fatalf("reoccupied source treated as verified move: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req), "new arrival")
	assertMoveBytes(t, moveNew(req), "source bytes")
}

func TestPreparedMoveReportsPostMutationUncertainty(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	result, err := m.execute(context.Background(), func() error {
		if err := moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName); err != nil {
			return err
		}
		writeMoveBytes(t, moveNew(req), "changed after mutation")
		return nil
	})
	if err == nil || !result.Attempted || !result.Completed || result.Verified || result.Target != nil {
		t.Fatalf("lost mutation outcome: %+v %v", result, err)
	}
	assertMoveBytes(t, moveNew(req), "changed after mutation")
}

func TestPreparedMoveInvalidPathsDoNotCreateDirectories(t *testing.T) {
	for _, relative := range []string{"", ".", "..", "../source.txt", "a/../b", "/absolute", "a//b", "a/", `a\b`, "a\x00b", strings.Repeat("a/", 65) + "b"} {
		t.Run(relative, func(t *testing.T) {
			req := moveFixture(t)
			req.NewRelative = relative
			if m, err := PrepareMove(context.Background(), req); err == nil {
				_ = m.Close()
				t.Fatalf("unsafe target accepted: %q", relative)
			}
			assertMoveBytes(t, moveOld(req), "source bytes")
		})
	}
	req := moveFixture(t)
	req.NewRelative = "missing/target.txt"
	if m, err := PrepareMove(context.Background(), req); err == nil {
		_ = m.Close()
		t.Fatal("missing parent accepted")
	}
	if _, err := os.Lstat(filepath.Join(req.DestinationRoot, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("prepare created directories")
	}
	for _, mutate := range []func(*MoveRequest){
		func(r *MoveRequest) { r.Source = nil },
		func(r *MoveRequest) { r.SourceDirectory = nil },
		func(r *MoveRequest) { r.DestinationDirectory = nil },
		func(r *MoveRequest) { r.DestinationRoot = "relative" },
		func(r *MoveRequest) { r.SourceRoot += string(filepath.Separator) + ".." },
	} {
		r := moveFixture(t)
		mutate(&r)
		if m, err := PrepareMove(context.Background(), r); err == nil {
			_ = m.Close()
			t.Fatal("unbound request accepted")
		}
	}
}

func TestPreparedMoveRejectsOccupiedAndLinkedPaths(t *testing.T) {
	for _, kind := range []string{"file", "directory", "dangling target", "source link", "target parent link", "source parent link", "root link", "ancestor link"} {
		t.Run(kind, func(t *testing.T) {
			req := moveFixture(t)
			link := func(target, path string) {
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			switch kind {
			case "file":
				writeMoveBytes(t, moveNew(req), "target bytes")
			case "directory":
				if err := os.Mkdir(moveNew(req), 0700); err != nil {
					t.Fatal(err)
				}
			case "dangling target":
				link(filepath.Join(req.DestinationRoot, "missing"), moveNew(req))
			case "source link":
				req.OldRelative = "nested/link"
				link(filepath.Join(req.SourceRoot, "nested", "source.txt"), moveOld(req))
			case "target parent link":
				req.NewRelative = "escape/target.txt"
				link(req.SourceRoot, filepath.Join(req.DestinationRoot, "escape"))
			case "source parent link":
				req.OldRelative = "escape/source.txt"
				link(filepath.Join(req.SourceRoot, "nested"), filepath.Join(req.SourceRoot, "escape"))
			case "root link":
				alias := filepath.Join(filepath.Dir(req.SourceRoot), "alias")
				link(req.SourceRoot, alias)
				req.SourceRoot = alias
			case "ancestor link":
				alias := filepath.Join(filepath.Dir(req.DestinationRoot), "alias")
				link(req.DestinationRoot, alias)
				req.DestinationRoot = filepath.Join(alias, "nested")
				var err error
				req.DestinationDirectory, err = Capture(filepath.Join(alias, "nested"))
				if err != nil {
					t.Fatal(err)
				}
				req.NewRelative = "target.txt"
			}
			if m, err := PrepareMove(context.Background(), req); err == nil {
				_ = m.Close()
				t.Fatalf("unsafe %s accepted", kind)
			}
			if kind == "file" {
				assertMoveBytes(t, moveNew(req), "target bytes")
			}
			if kind == "dangling target" {
				info, err := os.Lstat(moveNew(req))
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("dangling destination replaced")
				}
			}
		})
	}
}

func TestPreparedMoveRejectsChangedSourceAndRoots(t *testing.T) {
	for _, change := range []string{"file version", "source root", "destination parent"} {
		t.Run(change, func(t *testing.T) {
			req := moveFixture(t)
			m := prepareTestMove(t, req)
			switch change {
			case "file version":
				writeMoveBytes(t, moveOld(req), "changed bytes")
			default:
				path := req.SourceRoot
				if change == "destination parent" {
					path = filepath.Join(req.DestinationRoot, "nested")
				}
				if err := os.Rename(path, path+"-old"); err != nil {
					if runtime.GOOS != "windows" {
						t.Fatal(err)
					}
					if !errors.Is(err, syscall.Errno(32)) { // ERROR_SHARING_VIOLATION
						t.Fatalf("relocation did not fail because of pinned handles: %v", err)
					}
					if err := m.Check(context.Background()); err != nil {
						t.Fatal(err)
					}
					return
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			outcome, err := m.Execute(context.Background())
			if err == nil || outcome.Completed {
				t.Fatalf("changed namespace/source accepted: %+v %v", outcome, err)
			}
		})
	}
	req := moveFixture(t)
	if err := os.Rename(moveOld(req), moveOld(req)+"-old"); err != nil {
		t.Fatal(err)
	}
	writeMoveBytes(t, moveOld(req), "source bytes")
	if m, err := PrepareMove(context.Background(), req); !errors.Is(err, ErrStalePlan) {
		if m != nil {
			_ = m.Close()
		}
		t.Fatalf("replacement source accepted: %v", err)
	}
}

func TestPreparedMoveCancellationCloseAndConcurrentExecution(t *testing.T) {
	req := moveFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, err := PrepareMove(ctx, req); !errors.Is(err, context.Canceled) {
		if m != nil {
			_ = m.Close()
		}
		t.Fatalf("cancelled preparation: %v", err)
	}
	m := prepareTestMove(t, req)
	if result, err := m.Execute(ctx); !errors.Is(err, context.Canceled) || result.Completed {
		t.Fatalf("cancelled move: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Execute(context.Background()); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed move: %v", err)
	}
	var zero PreparedMove
	if err := zero.Check(context.Background()); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("zero move: %v", err)
	}
	_ = zero.Close()
	m = prepareTestMove(t, req)
	var wg sync.WaitGroup
	results := make(chan MoveOutcome, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); outcome, _ := m.Execute(context.Background()); results <- outcome }()
	}
	wg.Wait()
	close(results)
	completed := 0
	for outcome := range results {
		if outcome.Completed && outcome.Verified {
			completed++
		}
	}
	if completed != 1 {
		t.Fatalf("concurrent executions completed %d times", completed)
	}
	assertMoveBytes(t, moveNew(req), "source bytes")
}

func TestPreparedMoveCrossFilesystemRefusal(t *testing.T) {
	req := moveFixture(t)
	candidates := []string{"/dev/shm", "/"}
	if runtime.GOOS == "windows" {
		candidates = []string{`C:\`, `D:\`, `E:\`}
	}
	for _, path := range candidates {
		other, err := Capture(path)
		if err != nil || identityVolume(req.Source.Identity) == identityVolume(other.Identity) {
			continue
		}
		req.DestinationRoot, req.DestinationDirectory, req.NewRelative = path, other, "renym-rooted-move-fixture"
		m, err := PrepareMove(context.Background(), req)
		if m != nil {
			_ = m.Close()
		}
		if errors.Is(err, ErrNoReplaceUnsupported) {
			continue
		}
		if !errors.Is(err, ErrCrossFilesystem) {
			t.Fatalf("cross-filesystem move: %v", err)
		}
		assertMoveBytes(t, moveOld(req), "source bytes")
		return
	}
	t.Skip("no supported second filesystem available")
}

func TestPreparedMovePostCheckParentRelocationRequiresReconciliation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows directory handles deny delete sharing; relocation is tested before the final check")
	}
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	parent := filepath.Join(req.DestinationRoot, "nested")
	moved := filepath.Join(filepath.Dir(req.DestinationRoot), "relocated")
	result, err := m.execute(context.Background(), func() error {
		if err := os.Rename(parent, moved); err != nil {
			return err
		}
		if err := os.Symlink(req.SourceRoot, parent); err != nil {
			return err
		}
		return moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName)
	})
	if err == nil || !result.Completed || result.Verified || result.Target != nil {
		t.Fatalf("relocation race was reported as verified: %+v %v", result, err)
	}
	assertMoveBytes(t, filepath.Join(moved, "target.txt"), "source bytes")
	if _, err := os.Lstat(filepath.Join(req.SourceRoot, "target.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("move followed replacement symlink: %v", err)
	}
}

func TestPreparedMoveSourceReplacementAfterPreparation(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	old := moveOld(req)
	if err := os.Rename(old, old+"-original"); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		if !errors.Is(err, syscall.Errno(32)) { // ERROR_SHARING_VIOLATION
			t.Fatalf("replacement did not fail because of pinned source handle: %v", err)
		}
		assertMoveBytes(t, old, "source bytes")
		return
	}
	writeMoveBytes(t, old, "replacement bytes")
	result, err := m.Execute(context.Background())
	if !errors.Is(err, ErrStalePlan) || result.Attempted || result.Completed {
		t.Fatalf("replaced file moved: %+v %v", result, err)
	}
	assertMoveBytes(t, old, "replacement bytes")
	assertMoveBytes(t, old+"-original", "source bytes")
}

func TestPreparedMoveUsesOnlyExistingDirectoriesAndRegularFiles(t *testing.T) {
	req := moveFixture(t)
	req.SourceRoot += "-missing"
	if m, err := PrepareMove(context.Background(), req); err == nil {
		_ = m.Close()
		t.Fatal("missing root accepted")
	}
	if _, err := os.Lstat(req.SourceRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created input root")
	}
	req = moveFixture(t)
	req.OldRelative = "nested"
	req.Source, _ = Capture(filepath.Join(req.SourceRoot, "nested"))
	if m, err := PrepareMove(context.Background(), req); !errors.Is(err, ErrUnsafeMovePath) {
		if m != nil {
			_ = m.Close()
		}
		t.Fatalf("folder relocation accepted: %v", err)
	}
	req = moveFixture(t)
	original := req.DestinationRoot
	if err := os.Rename(original, original+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(original, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if m, err := PrepareMove(context.Background(), req); !errors.Is(err, ErrStalePlan) {
		if m != nil {
			_ = m.Close()
		}
		t.Fatalf("replaced planned root accepted: %v", err)
	}
}

func TestPreparedMoveCancellationAfterMutationKeepsOutcome(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := m.execute(ctx, func() error {
		if err := moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if err != nil || !result.Completed || !result.Verified {
		t.Fatalf("cancellation hid completed move: %+v %v", result, err)
	}
	assertMoveBytes(t, moveNew(req), "source bytes")
}

func TestPreparedMoveConcurrentCheckExecuteAndClose(t *testing.T) {
	req := moveFixture(t)
	for i := 0; i < 32; i++ {
		move := prepareTestMove(t, req)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var wg sync.WaitGroup
		for _, operation := range []func(){
			func() { _ = move.Check(context.Background()) },
			func() { _, _ = move.Execute(ctx) },
			func() { _ = move.Close() },
		} {
			wg.Add(1)
			go func() { defer wg.Done(); operation() }()
		}
		wg.Wait()
		if err := move.Check(context.Background()); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("close did not serialize: %v", err)
		}
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if _, err := os.Lstat(moveNew(req)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled race mutated destination: %v", err)
	}
}

func FuzzMoveRelativePath(f *testing.F) {
	for _, source := range []string{"source.txt", "nested/photo.png", "../escape", "/absolute", `C:\escape`, "", "a//b", "a\x00b", "CON", "emoji_😀.png"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		parts, err := moveComponents(source)
		if err != nil {
			return
		}
		local, err := filepath.Localize(source)
		if err != nil || !filepath.IsLocal(local) || len(parts) > 64 || len(source) > 4096 || strings.Join(parts, "/") != source {
			t.Fatalf("unsafe relative output: %q %v", source, err)
		}
		for _, part := range parts {
			if part == "." || part == ".." || part == "" {
				t.Fatal("dot/empty component accepted")
			}
		}
	})
}
