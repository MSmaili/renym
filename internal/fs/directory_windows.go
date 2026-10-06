//go:build windows

package fs

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func directoryOpenAt(parent *os.File, name string, remove bool) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	file, _, err := moveNTOpenWithOptions(windows.Handle(parent.Fd()), name, ntOpenOptions{directory: true, delete: remove, shareDelete: !remove})
	return file, err
}

func directoryCreateAt(parent *os.File, name string) (*os.File, bool, error) {
	defer runtime.KeepAlive(parent)
	return moveNTOpenWithOptions(windows.Handle(parent.Fd()), name, ntOpenOptions{directory: true, create: true})
}

func directoryRemoveAt(_ *os.File, _ string, file *os.File) error {
	defer runtime.KeepAlive(file)
	flags := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS)
	err := moveNTError(windows.NtSetInformationFile(windows.Handle(file.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)), windows.FileDispositionInformationEx))
	runtime.KeepAlive(&flags)
	if errors.Is(err, windows.ERROR_DIR_NOT_EMPTY) {
		return errors.Join(ErrDirectoryNotEmpty, err)
	}
	if errors.Is(err, windows.ERROR_NOT_SUPPORTED) || errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
		return errors.Join(ErrNoReplaceUnsupported, err)
	}
	return err
}

func directoryRemovalVerified(parent *os.File, name string) error {
	return moveEntryAbsent(parent, name)
}
