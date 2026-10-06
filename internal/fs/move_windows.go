//go:build windows

package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func moveAbsoluteParts(path string) (string, []string, error) {
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' || !((volume[0] >= 'A' && volume[0] <= 'Z') || (volume[0] >= 'a' && volume[0] <= 'z')) {
		return "", nil, ErrUnsafeMovePath
	}
	root := volume + `\`
	parts := strings.Split(strings.TrimPrefix(path, root), `\`)
	if path == root {
		parts = nil
	}
	for _, part := range parts {
		if err := ValidateName(part); err != nil {
			return "", nil, ErrUnsafeMovePath
		}
	}
	return root, parts, nil
}

func moveNTError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}

func moveNTOpen(parent windows.Handle, name string, directory, forMove bool) (*os.File, error) {
	utf16, err := windows.UTF16FromString(name)
	if err != nil {
		return nil, err
	}
	// Bound UTF-16 lengths before narrowing; avoid vulnerable NewNTUnicodeString.
	if len(utf16) > 32767 {
		return nil, ErrUnsafeMovePath
	}
	unicode := windows.NTUnicodeString{Length: uint16((len(utf16) - 1) * 2), MaximumLength: uint16(len(utf16) * 2), Buffer: &utf16[0]}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: &unicode, Attributes: windows.OBJ_CASE_INSENSITIVE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE)
	options := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if directory {
		access |= windows.FILE_TRAVERSE
		options |= windows.FILE_DIRECTORY_FILE
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
		if forMove {
			access |= windows.DELETE
		} else {
			share |= windows.FILE_SHARE_DELETE
		}
	}
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, access, &attrs, &windows.IO_STATUS_BLOCK{}, nil, 0, share, windows.FILE_OPEN, options, 0, 0)
	runtime.KeepAlive(utf16)
	if err != nil {
		return nil, moveNTError(err)
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(handle, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = ErrUnsafeMovePath
	}
	if err == nil {
		kind, kindErr := windows.GetFileType(handle)
		if kindErr != nil {
			err = kindErr
		} else if kind != windows.FILE_TYPE_DISK {
			err = ErrUnsafeMovePath
		}
	}
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return os.NewFile(uintptr(handle), name), nil
}

func moveOpenVolume(path string) (*os.File, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	kind := windows.GetDriveType(ptr)
	if kind != windows.DRIVE_FIXED && kind != windows.DRIVE_REMOVABLE && kind != windows.DRIVE_RAMDISK {
		return nil, ErrNoReplaceUnsupported
	}
	return moveNTOpen(0, `\??\`+path, true, false)
}

func moveOpenDirectoryAt(parent *os.File, name string) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	return moveNTOpen(windows.Handle(parent.Fd()), name, true, false)
}

func moveOpenFileAt(parent *os.File, name string, forMove bool) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	return moveNTOpen(windows.Handle(parent.Fd()), name, false, forMove)
}

func moveHandleIdentity(file *os.File, _ os.FileInfo) (string, error) {
	defer runtime.KeepAlive(file)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", info.VolumeSerialNumber, uint64(info.FileIndexHigh)<<32|uint64(info.FileIndexLow)), nil
}

func moveCheckLocal(_ *os.File) error { return nil }

func moveTargetAbsent(parent *os.File, name string) error {
	file, err := moveOpenFileAt(parent, name, false)
	if err == nil {
		_ = file.Close()
		return os.ErrExist
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

type moveRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [256]uint16
}

func newMoveRenameInformation(parent windows.Handle, name string) (*moveRenameInformation, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	utf16, err := windows.UTF16FromString(name)
	if err != nil {
		return nil, err
	}
	if len(utf16) > 256 {
		return nil, ErrUnsafeMovePath
	}
	info := &moveRenameInformation{RootDirectory: parent, FileNameLength: uint32((len(utf16) - 1) * 2)}
	copy(info.FileName[:], utf16)
	return info, nil
}

func moveRenameNoReplace(_ *os.File, _ string, source *os.File, to *os.File, name string) error {
	defer runtime.KeepAlive(source)
	defer runtime.KeepAlive(to)
	info, err := newMoveRenameInformation(windows.Handle(to.Fd()), name)
	if err != nil {
		return err
	}
	// ReplaceIfExists stays false; the target is one basename beneath the handle.
	err = windows.NtSetInformationFile(windows.Handle(source.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(info)), uint32(unsafe.Sizeof(*info)), windows.FileRenameInformation)
	runtime.KeepAlive(info)
	err = moveNTError(err)
	if errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) {
		return errors.Join(ErrCrossFilesystem, err)
	}
	if errors.Is(err, windows.ERROR_NOT_SUPPORTED) || errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
		return errors.Join(ErrNoReplaceUnsupported, err)
	}
	return err
}
