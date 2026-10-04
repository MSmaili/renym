package history

// Store defines the interface for history storage operations.
type Store interface {
	Directory() string
	Save(dirPath string, entry Entry) (string, error)

	Latest(dirPath string) (*Entry, error)

	Update(dirPath, historyFile string, entry Entry) error
	DeleteEntry(dirPath, historyFile string) error
}

type PathIdentifier interface {
	PathIdentifier(path string) (string, error)
}
