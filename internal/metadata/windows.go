//go:build windows

package metadata

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

type WindowsMetadataProvider struct{}

func NewMetadataProvider() MetadataProvider {
	return &WindowsMetadataProvider{}
}

func (p *WindowsMetadataProvider) GetMetadata(path string) (*FileMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get file size from standard stat
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Get Windows-specific file info for timestamps
	h := windows.Handle(f.Fd())
	var winInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &winInfo); err != nil {
		return nil, err
	}

	return &FileMetadata{
		ModTime:     time.Unix(0, winInfo.LastWriteTime.Nanoseconds()),
		CreatedTime: time.Unix(0, winInfo.CreationTime.Nanoseconds()),
		Size:        info.Size(),
		Extension:   filepath.Ext(path),
	}, nil
}
