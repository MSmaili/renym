package walker

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestWalk(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		cfg   Config
		want  []string
	}{
		{
			name:  "non-recursive only sees top-level files",
			files: []string{"file1.txt", "sub/file3.txt"},
			cfg: Config{
				Recursive: false,
				Files:     true,
			},
			want: []string{"file1.txt"},
		},
		{
			name: "recursive finds nested files and top level files no directiroes",
			files: []string{
				"sub/file2.txt",
				"file1.txt",
			},
			cfg: Config{
				Recursive: true,
				Files:     true,
			},
			want: []string{"sub/file2.txt", "file1.txt"},
		},
		{
			name: "ignore patterns skip files",
			files: []string{
				"a.go",
				"a.txt",
			},
			cfg: Config{
				Files:  true,
				Ignore: []string{"*.txt"},
			},
			want: []string{"a.go"},
		},
		{
			name: "ignore directory entirely",
			files: []string{
				"node_modules/x.js",
				"main.js",
			},
			cfg: Config{
				Files:  true,
				Ignore: []string{"node_modules"},
			},
			want: []string{"main.js"},
		},
		{
			name: "directories included when requested",
			files: []string{
				"dir/test2.txt",
				"dir2/file.txt",
			},
			cfg: Config{
				Recursive:   true,
				Directories: true,
				Files:       false,
			},
			want: []string{"dir", "dir2"},
		},
		{
			name: "by default it ignores .git directory",
			files: []string{
				".git/file1.txt",
				"dir/file.txt",
			},
			cfg: Config{
				Recursive:   true,
				Directories: true,
				Files:       true,
			},
			want: []string{"dir", "dir/file.txt"},
		},
		{
			name: "adds only directories",
			files: []string{
				".git/file1.txt",
				"dir/test/hello.txt",
			},
			cfg: Config{
				Recursive:   true,
				Directories: true,
				Files:       false,
			},
			want: []string{"dir", "dir/test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			createFiles(t, root, tt.files)

			cfg := tt.cfg
			cfg.Path = root

			got, err := Walk(cfg)
			assert.Nil(t, err)

			for i := range got {
				rel, _ := filepath.Rel(root, got[i])
				got[i] = rel
			}

			sort.Strings(got)
			sort.Strings(tt.want)

			assert.SliceEqual(t, got, tt.want)
		})
	}
}

func TestWalkSingleFile(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantCount int
	}{
		{
			name: "single file path with Files=true returns the file",
			cfg: Config{
				Files: true,
			},
			wantCount: 1,
		},
		{
			name: "single file path with Files=false returns empty",
			cfg: Config{
				Files: false,
			},
			wantCount: 0,
		},
		{
			name: "single file path with Directories=true and Files=false returns empty",
			cfg: Config{
				Directories: true,
				Files:       false,
			},
			wantCount: 0,
		},
		{
			name: "single file path with both flags returns the file",
			cfg: Config{
				Directories: true,
				Files:       true,
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			filePath := filepath.Join(root, "single-file.txt")
			err := os.WriteFile(filePath, []byte("content"), 0644)
			assert.Nil(t, err)

			cfg := tt.cfg
			cfg.Path = filePath

			got, err := Walk(cfg)
			assert.Nil(t, err)
			assert.Len(t, got, tt.wantCount)

			if tt.wantCount == 1 {
				assert.Equal(t, got[0], filePath)
			}
		})
	}
}

func TestIsFile(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(root string) string
		expected bool
	}{
		{
			name: "returns true for file",
			setup: func(root string) string {
				path := filepath.Join(root, "file.txt")
				os.WriteFile(path, []byte(""), 0644)
				return path
			},
			expected: true,
		},
		{
			name: "returns false for directory",
			setup: func(root string) string {
				path := filepath.Join(root, "dir")
				os.MkdirAll(path, 0755)
				return path
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := tt.setup(root)

			result, err := isFile(path)
			assert.Nil(t, err)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestIsFileNonExistent(t *testing.T) {
	_, err := isFile("/non/existent/path")
	assert.NotNil(t, err)
}

func createFiles(t *testing.T, root string, files []string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(root, file)

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to mkdir: %v", err)
		}

		if strings.HasSuffix(file, "/") {
			continue
		}

		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}
}
