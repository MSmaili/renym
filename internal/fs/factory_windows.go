//go:build windows

package fs

func NewAdapter() FileSystemAdapter {
	return WindowsFSAdapter{}
}
