//go:build linux || darwin

package fs

import (
	"errors"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func directoryOpenAt(parent *os.File, name string, _ bool) (*os.File, error) {
	return moveOpenDirectoryAt(parent, name)
}

func directoryCreateAt(parent *os.File, name string) (*os.File, bool, error) {
	defer runtime.KeepAlive(parent)
	if err := unix.Mkdirat(int(parent.Fd()), name, 0755); err != nil {
		return nil, false, err
	}
	file, err := moveOpenDirectoryAt(parent, name)
	return file, true, err
}

func directoryRemoveAt(parent *os.File, name string, file *os.File) error {
	defer runtime.KeepAlive(parent)
	defer runtime.KeepAlive(file)
	err := unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
	if errors.Is(err, unix.ENOTEMPTY) || errors.Is(err, unix.EEXIST) {
		return errors.Join(ErrDirectoryNotEmpty, err)
	}
	return err
}

func directoryRemovalVerified(parent *os.File, name string) error {
	return moveEntryAbsent(parent, name)
}
