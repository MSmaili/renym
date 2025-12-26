//go:build darwin

package metadata

import (
	"syscall"
	"time"
)

// getCreatedTime returns the file creation time on macOS
// Falls back to modTime if Birthtimespec is zero
func getCreatedTime(stat *syscall.Stat_t, modTime time.Time) time.Time {
	if stat.Birthtimespec.Sec == 0 && stat.Birthtimespec.Nsec == 0 {
		return modTime
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

// getCreatedTimeWithPath returns the file creation time on macOS
// Path is not needed on macOS since Birthtimespec is always available
func getCreatedTimeWithPath(path string, stat *syscall.Stat_t, modTime time.Time) time.Time {
	return getCreatedTime(stat, modTime)
}
