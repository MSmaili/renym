//go:build linux

package metadata

import (
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// getCreatedTime returns the file creation time on Linux
// Uses statx() syscall (kernel 4.11+) for true birth time,
// falls back to Ctim (change time) on older systems
func getCreatedTime(stat *syscall.Stat_t, modTime time.Time) time.Time {
	// Ctim fallback - "change time" when metadata was last changed
	if stat.Ctim.Sec == 0 && stat.Ctim.Nsec == 0 {
		return modTime
	}
	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}

// getCreatedTimeWithPath attempts to get true birth time using statx()
// Falls back to getCreatedTime if statx() is unavailable or doesn't provide birth time
func getCreatedTimeWithPath(path string, stat *syscall.Stat_t, modTime time.Time) time.Time {
	var statx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &statx)
	if err == nil && statx.Mask&unix.STATX_BTIME != 0 {
		return time.Unix(statx.Btime.Sec, int64(statx.Btime.Nsec))
	}

	// Fall back to Ctim
	return getCreatedTime(stat, modTime)
}
