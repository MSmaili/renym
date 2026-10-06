//go:build !linux && !darwin && !windows

package fs

import "os"

func moveAbsoluteParts(string) (string, []string, error)       { return "", nil, ErrNoReplaceUnsupported }
func moveOpenVolume(string) (*os.File, error)                  { return nil, ErrNoReplaceUnsupported }
func moveOpenDirectoryAt(*os.File, string) (*os.File, error)   { return nil, ErrNoReplaceUnsupported }
func moveOpenFileAt(*os.File, string, bool) (*os.File, error)  { return nil, ErrNoReplaceUnsupported }
func moveHandleIdentity(*os.File, os.FileInfo) (string, error) { return "", ErrNoReplaceUnsupported }
func moveCheckLocal(*os.File) error                            { return ErrNoReplaceUnsupported }
func moveEntryAbsent(*os.File, string) error                   { return ErrNoReplaceUnsupported }
func moveRenameNoReplace(*os.File, string, *os.File, *os.File, string) error {
	return ErrNoReplaceUnsupported
}
