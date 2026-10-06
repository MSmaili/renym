//go:build darwin

package fs

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func moveRenameNoReplace(from *os.File, old string, _ *os.File, to *os.File, name string) error {
	defer runtime.KeepAlive(from)
	defer runtime.KeepAlive(to)
	return moveUnixRenameError(unix.RenameatxNp(int(from.Fd()), old, int(to.Fd()), name, unix.RENAME_EXCL|unix.RENAME_NOFOLLOW_ANY))
}

func moveCheckLocal(file *os.File) error {
	defer runtime.KeepAlive(file)
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &stat); err != nil {
		return err
	}
	if stat.Flags&unix.MNT_LOCAL == 0 {
		return ErrNoReplaceUnsupported
	}
	return nil
}
