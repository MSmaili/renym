package fs

func NewAdapter() FileSystemAdapter {
	return &UnixFSAdapter{}
}
