package history

// Store defines the interface for history storage operations.
type Store interface {
	Save(dirPath string, entry Entry) (string, error)

	Latest(dirPath string) (*Entry, error)

	Delete(dirPath string, historyFile string) error
}

type PathIdentifier interface {
	PathIdentifier(path string) (string, error)
}
