package walker

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
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
			// Build temp tree
			root := t.TempDir()
			createFiles(t, root, tt.files)

			// Set root path
			cfg := tt.cfg
			cfg.Path = root

			got, err := Walk(cfg)
			if err != nil {
				t.Fatalf("Walk error: %v", err)
			}

			// Convert full paths → relative paths for easy comparison
			for i := range got {
				rel, _ := filepath.Rel(root, got[i])
				got[i] = rel
			}

			// Sorting so order doesn’t matter
			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected result.\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
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
