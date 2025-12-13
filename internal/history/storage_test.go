package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestSave(t *testing.T) {
	tests := []struct {
		name      string
		entry     Entry
		wantError bool
	}{
		{
			name: "save valid entry",
			entry: Entry{
				Version:   "1.0",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Path:      "/test/path",
				Command:   "rename",
				Operations: []Operation{
					{Old: "file1.txt", New: "file1_new.txt"},
				},
			},
			wantError: false,
		},
		{
			name: "save entry with empty operations",
			entry: Entry{
				Version:    "1.0",
				Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Path:       "/test/path",
				Command:    "rename",
				Operations: []Operation{},
			},
			wantError: false,
		},
		{
			name: "save entry with multiple operations",
			entry: Entry{
				Version:   "1.0",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Path:      "/test/path",
				Command:   "rename",
				Operations: []Operation{
					{Old: "file1.txt", New: "file1_new.txt"},
					{Old: "file2.txt", New: "file2_new.txt"},
					{Old: "file3.txt", New: "file3_new.txt"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			err := Save(tmpDir, tt.entry)

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)

				histDir := filepath.Join(tmpDir, historyDir)
				files, err := os.ReadDir(histDir)
				assert.Nil(t, err)
				assert.Equal(t, len(files) > 0, true)
			}
		})
	}
}

func TestSaveWithFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "some-file.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	assert.Nil(t, err)

	entry := Entry{
		Version:   "1.0",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Path:      filePath,
		Command:   "rename",
	}

	err = Save(filePath, entry)
	assert.Nil(t, err)

	// History should be saved in parent directory, not inside the file
	histDir := filepath.Join(tmpDir, historyDir)
	files, err := os.ReadDir(histDir)
	assert.Nil(t, err)
	assert.Equal(t, len(files) > 0, true)
}

func TestResolveDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(root string) string
		wantDir bool
		wantErr bool
	}{
		{
			name: "returns directory path unchanged",
			setup: func(root string) string {
				return root
			},
			wantDir: true,
			wantErr: false,
		},
		{
			name: "returns parent directory for file path",
			setup: func(root string) string {
				filePath := filepath.Join(root, "file.txt")
				os.WriteFile(filePath, []byte(""), 0644)
				return filePath
			},
			wantDir: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := tt.setup(root)

			result, err := resolveDir(path)

			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				if tt.wantDir {
					assert.Equal(t, result, path)
				} else {
					assert.Equal(t, result, filepath.Dir(path))
				}
			}
		})
	}
}

func TestResolveDirNonExistent(t *testing.T) {
	_, err := resolveDir("/non/existent/path")
	assert.NotNil(t, err)
}

func TestList(t *testing.T) {
	tests := []struct {
		name          string
		setupFiles    []string
		expectedCount int
		wantError     bool
	}{
		{
			name:          "list empty directory",
			setupFiles:    []string{},
			expectedCount: 0,
			wantError:     false,
		},
		{
			name:          "list single file",
			setupFiles:    []string{"2024-01-15_103000.json"},
			expectedCount: 1,
			wantError:     false,
		},
		{
			name: "list multiple files sorted descending",
			setupFiles: []string{
				"2024-01-15_103000.json",
				"2024-01-16_103000.json",
				"2024-01-14_103000.json",
			},
			expectedCount: 3,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, _ := os.Getwd()
			tmpDir := t.TempDir()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			if len(tt.setupFiles) > 0 {
				os.MkdirAll(historyDir, 0755)
				for _, f := range tt.setupFiles {
					os.WriteFile(filepath.Join(historyDir, f), []byte("{}"), 0644)
				}
			}

			files, err := List()

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, len(files), tt.expectedCount)

				if len(files) > 1 {
					assert.Equal(t, files[0] > files[1], true)
				}
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	tests := []struct {
		name           string
		setupFiles     []string
		keepLast       int
		expectedRemain int
		wantError      bool
	}{
		{
			name:           "cleanup with no files",
			setupFiles:     []string{},
			keepLast:       5,
			expectedRemain: 0,
			wantError:      false,
		},
		{
			name: "cleanup keeps all when under limit",
			setupFiles: []string{
				"2024-01-15_103000.json",
				"2024-01-16_103000.json",
			},
			keepLast:       5,
			expectedRemain: 2,
			wantError:      false,
		},
		{
			name: "cleanup removes old files when over limit",
			setupFiles: []string{
				"2024-01-15_103000.json",
				"2024-01-16_103000.json",
				"2024-01-17_103000.json",
				"2024-01-18_103000.json",
				"2024-01-19_103000.json",
			},
			keepLast:       2,
			expectedRemain: 2,
			wantError:      false,
		},
		{
			name: "cleanup keeps exactly keepLast files",
			setupFiles: []string{
				"2024-01-15_103000.json",
				"2024-01-16_103000.json",
				"2024-01-17_103000.json",
			},
			keepLast:       3,
			expectedRemain: 3,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, _ := os.Getwd()
			tmpDir := t.TempDir()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			if len(tt.setupFiles) > 0 {
				os.MkdirAll(historyDir, 0755)
				for _, f := range tt.setupFiles {
					os.WriteFile(filepath.Join(historyDir, f), []byte("{}"), 0644)
				}
			}

			err := Cleanup(tt.keepLast)

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				files, _ := List()
				assert.Equal(t, len(files), tt.expectedRemain)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   map[string]string
		loadPath     string
		wantVersion  string
		wantError    bool
		wantOpsOrder []string // expected order of Operation.Old paths (if non-empty)
	}{
		{
			name: "load specific file",
			setupFiles: map[string]string{
				"2024-01-15_103000.json": `{"version":"1.0","timestamp":"2024-01-15T10:30:00Z","path":"/test","command":"rename","operations":[]}`,
			},
			loadPath:    filepath.Join(historyDir, "2024-01-15_103000.json"),
			wantVersion: "1.0",
			wantError:   false,
		},
		{
			name: "load latest when path is empty",
			setupFiles: map[string]string{
				"2024-01-14_103000.json": `{"version":"1.0","timestamp":"2024-01-14T10:30:00Z","path":"/test","command":"rename","operations":[]}`,
				"2024-01-15_103000.json": `{"version":"2.0","timestamp":"2024-01-15T10:30:00Z","path":"/test","command":"rename","operations":[]}`,
			},
			loadPath:    "",
			wantVersion: "2.0",
			wantError:   false,
		},
		{
			name:       "load non-existent file",
			setupFiles: map[string]string{},
			loadPath:   "non-existent.json",
			wantError:  true,
		},
		{
			name: "load invalid json",
			setupFiles: map[string]string{
				"2024-01-15_103000.json": `{invalid json}`,
			},
			loadPath:  filepath.Join(historyDir, "2024-01-15_103000.json"),
			wantError: true,
		},
		{
			name: "load sorts operations by depth",
			setupFiles: map[string]string{
				"2024-01-15_103000.json": `{"version":"1.0","timestamp":"2024-01-15T10:30:00Z","path":"/test","command":"rename","operations":[
					{"old":"a/b/c/deep.txt","new":"a/b/c/deep_new.txt"},
					{"old":"a/mid.txt","new":"a/mid_new.txt"},
					{"old":"top.txt","new":"top_new.txt"},
					{"old":"a/b/nested.txt","new":"a/b/nested_new.txt"}
				]}`,
			},
			loadPath:     filepath.Join(historyDir, "2024-01-15_103000.json"),
			wantVersion:  "1.0",
			wantError:    false,
			wantOpsOrder: []string{"top.txt", "a/mid.txt", "a/b/nested.txt", "a/b/c/deep.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, _ := os.Getwd()
			tmpDir := t.TempDir()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			if len(tt.setupFiles) > 0 {
				os.MkdirAll(historyDir, 0755)
				for name, content := range tt.setupFiles {
					os.WriteFile(filepath.Join(historyDir, name), []byte(content), 0644)
				}
			}

			entry, err := Load(tt.loadPath)

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, entry)
				assert.Equal(t, entry.Version, tt.wantVersion)

				if len(tt.wantOpsOrder) > 0 {
					assert.Equal(t, len(entry.Operations), len(tt.wantOpsOrder))
					for i, wantOld := range tt.wantOpsOrder {
						assert.Equal(t, entry.Operations[i].Old, wantOld)
					}
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles []string
		deletePath string
		wantError  bool
	}{
		{
			name: "delete specific file",
			setupFiles: []string{
				"2024-01-15_103000.json",
			},
			deletePath: filepath.Join(historyDir, "2024-01-15_103000.json"),
			wantError:  false,
		},
		{
			name: "delete latest when path is empty",
			setupFiles: []string{
				"2024-01-14_103000.json",
				"2024-01-15_103000.json",
			},
			deletePath: "",
			wantError:  false,
		},
		{
			name:       "delete non-existent file",
			setupFiles: []string{},
			deletePath: "non-existent.json",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, _ := os.Getwd()
			tmpDir := t.TempDir()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			if len(tt.setupFiles) > 0 {
				os.MkdirAll(historyDir, 0755)
				for _, f := range tt.setupFiles {
					os.WriteFile(filepath.Join(historyDir, f), []byte("{}"), 0644)
				}
			}

			initialCount := len(tt.setupFiles)
			err := Delete(tt.deletePath)

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				files, _ := List()
				assert.Equal(t, len(files), initialCount-1)
			}
		})
	}
}

func TestLatest(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   []string
		expectedFile string
		wantError    bool
	}{
		{
			name:       "empty directory returns error",
			setupFiles: []string{},
			wantError:  true,
		},
		{
			name: "returns latest file by name",
			setupFiles: []string{
				"2024-01-14_103000.json",
				"2024-01-16_103000.json",
				"2024-01-15_103000.json",
			},
			expectedFile: "2024-01-16_103000.json",
			wantError:    false,
		},
		{
			name: "single file",
			setupFiles: []string{
				"2024-01-15_103000.json",
			},
			expectedFile: "2024-01-15_103000.json",
			wantError:    false,
		},
		{
			name: "ignores non-json files",
			setupFiles: []string{
				"2024-01-14_103000.json",
				"2024-01-17_103000.txt",
			},
			expectedFile: "2024-01-14_103000.json",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			histDir := filepath.Join(tmpDir, historyDir)

			if len(tt.setupFiles) > 0 {
				os.MkdirAll(histDir, 0755)
				for _, f := range tt.setupFiles {
					os.WriteFile(filepath.Join(histDir, f), []byte("{}"), 0644)
				}
			} else {
				os.MkdirAll(histDir, 0755)
			}

			result, err := latest(histDir)

			if tt.wantError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, result, tt.expectedFile)
			}
		})
	}
}
