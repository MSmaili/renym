//go:build linux || darwin

package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func openMoveTestFile(path string, write bool) (*os.File, error) {
	if write {
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	}
	return os.Open(path)
}

func TestPreparedMoveRejectsFIFOWithoutBlocking(t *testing.T) {
	req := moveFixture(t)
	old := moveOld(req)
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(old, 0600); err != nil {
		t.Fatal(err)
	}
	if m, err := PrepareMove(context.Background(), req); err == nil {
		_ = m.Close()
		t.Fatal("FIFO accepted")
	}
}

func TestMoveUnsupportedNativeErrorsNeverBecomeFallbacks(t *testing.T) {
	for _, native := range []error{unix.ENOSYS, unix.EINVAL, unix.ENOTSUP} {
		if err := moveUnixRenameError(native); !errors.Is(err, ErrNoReplaceUnsupported) || !errors.Is(err, native) {
			t.Fatalf("unsupported cause lost: %v", err)
		}
	}
	if err := moveUnixRenameError(unix.EXDEV); !errors.Is(err, ErrCrossFilesystem) {
		t.Fatal("cross-device cause lost")
	}
}

func TestPreparedMovePostCheckSourceReplacementRequiresReconciliation(t *testing.T) {
	req := moveFixture(t)
	m := prepareTestMove(t, req)
	result, err := m.execute(context.Background(), func() error {
		if err := os.Rename(moveOld(req), moveOld(req)+"-original"); err != nil {
			return err
		}
		writeMoveBytes(t, moveOld(req), "replacement bytes")
		return moveRenameNoReplace(m.from.parent(), m.oldName, m.source, m.to.parent(), m.newName)
	})
	if err == nil || !result.Completed || result.Verified || result.Target != nil {
		t.Fatalf("source race lost uncertainty: %+v %v", result, err)
	}
	assertMoveBytes(t, moveOld(req)+"-original", "source bytes")
	assertMoveBytes(t, moveNew(req), "replacement bytes")
}

func TestPreparedMoveRepeatedPreparationReleasesDescriptors(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("Linux descriptor inventory fixture")
	}
	req := moveFixture(t)
	count := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	m := prepareTestMove(t, req)
	_ = m.Close()
	before := count()
	for i := 0; i < 32; i++ {
		m, err := PrepareMove(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
		bad := req
		bad.NewRelative = "missing/target.txt"
		if m, err := PrepareMove(context.Background(), bad); err == nil {
			_ = m.Close()
			t.Fatal("missing parent accepted")
		}
	}
	if after := count(); after != before {
		t.Fatalf("descriptor leak: before %d, after %d", before, after)
	}
	if _, err := os.Stat(filepath.Join(req.DestinationRoot, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failure created output directories")
	}
}
