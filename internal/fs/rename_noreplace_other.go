//go:build !darwin && !linux && !windows

package fs

func renameNoReplace(oldPath, newPath string) error {
	return ErrNoReplaceUnsupported
}
