//go:build windows

package fs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openMoveTestFile(path string, write bool) (*os.File, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access, disposition := uint32(windows.GENERIC_READ), uint32(windows.OPEN_EXISTING)
	if write {
		access, disposition = windows.GENERIC_WRITE, windows.CREATE_ALWAYS
	}
	// Test I/O must share delete access with the pinned source handle.
	handle, err := windows.CreateFile(ptr, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, disposition, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func TestPreparedMoveRejectsWindowsJunction(t *testing.T) {
	req := moveFixture(t)
	junction := filepath.Join(req.DestinationRoot, "junction")
	output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, req.SourceRoot).CombinedOutput()
	if err != nil {
		t.Fatalf("create disposable junction: %v: %s", err, output)
	}
	req.NewRelative = "junction/target.txt"
	if m, err := PrepareMove(context.Background(), req); err == nil {
		_ = m.Close()
		t.Fatal("junction was traversed")
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	if _, err := os.Lstat(filepath.Join(req.SourceRoot, "target.txt")); !os.IsNotExist(err) {
		t.Fatal("junction escaped into input")
	}
}

func TestMoveRenameInformationNeverEnablesReplace(t *testing.T) {
	name := "photo_😀.png"
	info, err := newMoveRenameInformation(windows.Handle(123), name)
	if err != nil {
		t.Fatal(err)
	}
	utf16, _ := windows.UTF16FromString(name)
	if info.ReplaceIfExists != 0 || info.RootDirectory != windows.Handle(123) || info.FileNameLength != uint32((len(utf16)-1)*2) || windows.UTF16ToString(info.FileName[:]) != name {
		t.Fatal("rename defaults to overwrite")
	}
	rootOffset := unsafe.Sizeof(windows.Handle(0))
	if unsafe.Offsetof(info.RootDirectory) != rootOffset || unsafe.Offsetof(info.FileNameLength) != rootOffset*2 || unsafe.Offsetof(info.FileName) != rootOffset*2+4 {
		t.Fatal("native rename information layout mismatch")
	}
	for _, bad := range []string{"..", ".", `a\b`, "a/b", "a:stream", strings.Repeat("a", 256)} {
		if _, err := newMoveRenameInformation(windows.Handle(123), bad); err == nil {
			t.Fatalf("invalid native name accepted: %q", bad)
		}
	}
	if _, _, err := moveAbsoluteParts(`\\server\share\folder`); err == nil {
		t.Fatal("UNC root accepted")
	}
	if _, _, err := moveAbsoluteParts(`\\?\C:\folder`); err == nil {
		t.Fatal("device namespace root accepted")
	}
}

func TestMoveNTOpenRejectsUnicodeLengthOverflowBeforeSyscall(t *testing.T) {
	for _, name := range []string{strings.Repeat("a", 32767), strings.Repeat("😀", 16384)} {
		file, err := moveNTOpen(0, name, false, false)
		if file != nil {
			_ = file.Close()
		}
		if !errors.Is(err, ErrUnsafeMovePath) {
			t.Fatalf("overflowed native unicode length: %v", err)
		}
	}
}

func TestPreparedMoveRejectsWindowsHardLinkedSource(t *testing.T) {
	req := moveFixture(t)
	alias := moveOld(req) + "-alias"
	if err := os.Link(moveOld(req), alias); err != nil {
		t.Fatal(err)
	}
	move, err := PrepareMove(context.Background(), req)
	if move != nil {
		_ = move.Close()
	}
	if !errors.Is(err, ErrHardLinkedSource) {
		t.Fatalf("hard-linked source accepted: %v", err)
	}
	assertMoveBytes(t, moveOld(req), "source bytes")
	assertMoveBytes(t, alias, "source bytes")
	if _, err := os.Lstat(moveNew(req)); !os.IsNotExist(err) {
		t.Fatal("hard-link refusal created destination")
	}
}
