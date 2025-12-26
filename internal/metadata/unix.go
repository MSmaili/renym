//go:build !windows

package metadata

import (
	"os"
	"path/filepath"
	"syscall"
)

type UnixMetadataProvider struct{}

func NewMetadataProvider() MetadataProvider {
	return &UnixMetadataProvider{}
}

func (p *UnixMetadataProvider) GetMetadata(path string) (*FileMetadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	modTime := info.ModTime()
	createdTime := modTime // default fallback

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		createdTime = getCreatedTimeWithPath(path, stat, modTime)
	}

	return &FileMetadata{
		ModTime:     modTime,
		CreatedTime: createdTime,
		Size:        info.Size(),
		Extension:   filepath.Ext(path),
	}, nil
}
