//go:build linux || darwin

package fs

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestDirectoryPreparationAndFailureReleaseDescriptors(t *testing.T) {
	fdPath := "/proc/self/fd"
	if runtime.GOOS == "darwin" {
		fdPath = "/dev/fd"
	}
	count := func() int {
		entries, err := os.ReadDir(fdPath)
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	req := directoryFixture(t)
	owned := createTestDirectory(t, req)
	remove, err := PrepareDirectoryRemoval(context.Background(), owned)
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Close(); err != nil {
		t.Fatal(err)
	}
	before := count()
	for i := 0; i < 32; i++ {
		remove, err := PrepareDirectoryRemoval(context.Background(), owned)
		if err != nil {
			t.Fatal(err)
		}
		if err := remove.Close(); err != nil {
			t.Fatal(err)
		}
		if directory, err := PrepareDirectoryCreation(context.Background(), req); err == nil {
			_ = directory.Close()
			t.Fatal("occupied directory accepted")
		}
		bad := req
		bad.Relative = "missing/child"
		if directory, err := PrepareDirectoryCreation(context.Background(), bad); err == nil {
			_ = directory.Close()
			t.Fatal("missing parent accepted")
		}
	}
	if after := count(); after != before {
		t.Fatalf("directory descriptor leak: before=%d after=%d", before, after)
	}
}
