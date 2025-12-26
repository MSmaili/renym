package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestGetMetadataValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("hello world")
	err := os.WriteFile(tmpFile, content, 0644)
	assert.Nil(t, err)

	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata(tmpFile)

	assert.Nil(t, err)
	assert.NotNil(t, meta)
}

func TestGetMetadataNonExistentFile(t *testing.T) {
	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata("/nonexistent/path/file.txt")

	assert.NotNil(t, err)
	assert.True(t, meta == nil, "metadata should be nil for non-existent file")
}

func TestGetMetadataExtension(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		wantExt  string
	}{
		{"simple", "file.txt", ".txt"},
		{"image", "photo.jpg", ".jpg"},
		{"compressed", "archive.tar.gz", ".gz"},
		{"noext", "README", ""},
		{"dotfile", ".gitignore", ".gitignore"},
		{"multiext", "app.config.json", ".json"},
	}

	provider := NewMetadataProvider()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(tmpFile, []byte("test"), 0644)
			assert.Nil(t, err)

			meta, err := provider.GetMetadata(tmpFile)
			assert.Nil(t, err)
			assert.Equal(t, meta.Extension, tt.wantExt)
		})
	}
}

func TestGetMetadataSize(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("hello world")
	err := os.WriteFile(tmpFile, content, 0644)
	assert.Nil(t, err)

	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata(tmpFile)

	assert.Nil(t, err)
	assert.Equal(t, meta.Size, int64(len(content)))
}

func TestGetMetadataSizeEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.txt")

	err := os.WriteFile(tmpFile, []byte{}, 0644)
	assert.Nil(t, err)

	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata(tmpFile)

	assert.Nil(t, err)
	assert.Equal(t, meta.Size, int64(0))
}

func TestGetMetadataTimes(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	before := time.Now().Add(-time.Second)
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	assert.Nil(t, err)
	after := time.Now().Add(time.Second)

	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata(tmpFile)

	assert.Nil(t, err)

	// ModTime should be between before and after
	assert.True(t, !meta.ModTime.Before(before), "ModTime should be after start time")
	assert.True(t, !meta.ModTime.After(after), "ModTime should be before end time")

	// CreatedTime should not be zero
	assert.True(t, !meta.CreatedTime.IsZero(), "CreatedTime should not be zero")

	// CreatedTime should be <= ModTime (or very close)
	assert.True(t, !meta.CreatedTime.After(meta.ModTime.Add(time.Second)), "CreatedTime should not be after ModTime")
}

func TestGetMetadataDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	provider := NewMetadataProvider()
	meta, err := provider.GetMetadata(tmpDir)

	assert.Nil(t, err)
	assert.NotNil(t, meta)
	assert.Equal(t, meta.Extension, "")
}
