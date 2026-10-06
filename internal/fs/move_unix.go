//go:build linux || darwin

package fs

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func moveAbsoluteParts(path string) (string, []string, error) {
	if strings.HasPrefix(path, "//") {
		return "", nil, ErrUnsafeMovePath
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if path == "/" {
		parts = nil
	}
	for _, part := range parts {
		if err := ValidateName(part); err != nil {
			return "", nil, ErrUnsafeMovePath
		}
	}
	return "/", parts, nil
}

func moveOpenVolume(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func moveOpenDirectoryAt(parent *os.File, name string) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func moveOpenFileAt(parent *os.File, name string, _ bool) (*os.File, error) {
	defer runtime.KeepAlive(parent)
	// O_NONBLOCK prevents FIFO validation from hanging.
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = ErrUnsafeMovePath
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func moveHandleIdentity(_ *os.File, info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ErrNoReplaceUnsupported
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

func moveTargetAbsent(parent *os.File, name string) error {
	defer runtime.KeepAlive(parent)
	var stat unix.Stat_t
	err := unix.Fstatat(int(parent.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		return os.ErrExist
	}
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

func moveUnixRenameError(err error) error {
	if errors.Is(err, unix.EXDEV) {
		return errors.Join(ErrCrossFilesystem, err)
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTSUP) {
		return errors.Join(ErrNoReplaceUnsupported, err)
	}
	return err
}
