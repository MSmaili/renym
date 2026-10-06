//go:build linux

package fs

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func moveRenameNoReplace(from *os.File, old string, _ *os.File, to *os.File, name string) error {
	defer runtime.KeepAlive(from)
	defer runtime.KeepAlive(to)
	return moveUnixRenameError(unix.Renameat2(int(from.Fd()), old, int(to.Fd()), name, unix.RENAME_NOREPLACE))
}

func moveCheckLocal(file *os.File) error {
	defer runtime.KeepAlive(file)
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &stat); err != nil {
		return err
	}
	// Unknown/network/userspace filesystems fail closed.
	switch uint64(stat.Type) {
	case unix.EXT4_SUPER_MAGIC, unix.TMPFS_MAGIC, unix.OVERLAYFS_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC, unix.F2FS_SUPER_MAGIC, unix.MSDOS_SUPER_MAGIC, unix.EXFAT_SUPER_MAGIC:
		return nil
	default:
		return ErrNoReplaceUnsupported
	}
}
