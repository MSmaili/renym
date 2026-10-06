//go:build !linux && !darwin && !windows

package fs

import "os"

func directoryOpenAt(*os.File, string, bool) (*os.File, error) { return nil, ErrNoReplaceUnsupported }
func directoryCreateAt(*os.File, string) (*os.File, bool, error) {
	return nil, false, ErrNoReplaceUnsupported
}
func directoryRemoveAt(*os.File, string, *os.File) error { return ErrNoReplaceUnsupported }
func directoryRemovalVerified(*os.File, string) error    { return ErrNoReplaceUnsupported }
