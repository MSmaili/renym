package metadata

import "time"

type FileMetadata struct {
	// ModifiedTime
	ModTime time.Time
	// CreatedTime -> not 100% accurate on linux systems
	CreatedTime time.Time
	Size        int64
	// FileExtension
	Extension string
}

type MetadataProvider interface {
	GetMetadata(path string) (*FileMetadata, error)
}
