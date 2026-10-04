//go:build windows

package fs

import "golang.org/x/sys/windows"

func renameNoReplace(oldPath, newPath string) error {
	oldPtr, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPtr, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	// No REPLACE_EXISTING or COPY_ALLOWED: fail instead of overwriting/copying.
	return windows.MoveFileEx(oldPtr, newPtr, 0)
}
