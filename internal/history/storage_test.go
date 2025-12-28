package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/common/testutils/assert"
)

type mockPathIdentifier struct {
	ids map[string]string
	err error
}

func (m *mockPathIdentifier) PathIdentifier(path string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if id, ok := m.ids[path]; ok {
		return id, nil
	}
	return path, nil
}

func newTestStore(t *testing.T, pathID PathIdentifier) (*GlobalStore, string) {
	t.Helper()
	tmpDir := t.TempDir()

	store := &GlobalStore{
		configDir: tmpDir,
		pathID:    pathID,
	}
	return store, tmpDir
}

func TestSanitizeDirID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no colons", "simple-path", "simple-path"},
		{"single colon", "C:/path", "C_/path"},
		{"multiple colons", "a:b:c:d", "a_b_c_d"},
		{"mixed path", "C:/Users/test:file", "C_/Users/test_file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeDirID(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestSortOperationsByDepth(t *testing.T) {
	tests := []struct {
		name     string
		input    []Operation
		expected []Operation
	}{
		{
			name:     "empty operations",
			input:    []Operation{},
			expected: []Operation{},
		},
		{
			name: "single operation",
			input: []Operation{
				{Old: "a/b/c", New: "x/y/z"},
			},
			expected: []Operation{
				{Old: "a/b/c", New: "x/y/z"},
			},
		},
		{
			name: "already sorted by depth",
			input: []Operation{
				{Old: "a", New: "b"},
				{Old: "a/b", New: "c/d"},
				{Old: "a/b/c", New: "x/y/z"},
			},
			expected: []Operation{
				{Old: "a", New: "b"},
				{Old: "a/b", New: "c/d"},
				{Old: "a/b/c", New: "x/y/z"},
			},
		},
		{
			name: "reverse order needs sorting",
			input: []Operation{
				{Old: "a/b/c/d", New: "w"},
				{Old: "a/b/c", New: "x"},
				{Old: "a/b", New: "y"},
				{Old: "a", New: "z"},
			},
			expected: []Operation{
				{Old: "a", New: "z"},
				{Old: "a/b", New: "y"},
				{Old: "a/b/c", New: "x"},
				{Old: "a/b/c/d", New: "w"},
			},
		},
		{
			name: "mixed depths",
			input: []Operation{
				{Old: "a/b", New: "1"},
				{Old: "x", New: "2"},
				{Old: "p/q/r", New: "3"},
				{Old: "m/n", New: "4"},
			},
			expected: []Operation{
				{Old: "x", New: "2"},
				{Old: "a/b", New: "1"},
				{Old: "m/n", New: "4"},
				{Old: "p/q/r", New: "3"},
			},
		},
		{
			name: "same depth preserved order",
			input: []Operation{
				{Old: "a/b", New: "1"},
				{Old: "c/d", New: "2"},
				{Old: "e/f", New: "3"},
			},
			expected: []Operation{
				{Old: "a/b", New: "1"},
				{Old: "c/d", New: "2"},
				{Old: "e/f", New: "3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := make([]Operation, len(tt.input))
			copy(ops, tt.input)

			sortOperationsByDepth(ops)

			assert.Len(t, ops, len(tt.expected))
			for i := range ops {
				assert.Equal(t, ops[i].Old, tt.expected[i].Old)
				assert.Equal(t, ops[i].New, tt.expected[i].New)
			}
		})
	}
}

func TestGlobalStoreSave(t *testing.T) {
	tests := []struct {
		name        string
		dirPath     string
		entry       Entry
		pathIDs     map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:    "save simple entry",
			dirPath: "",
			entry: Entry{
				Version:   "1.0",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
				Command:   "rename",
				Operations: []Operation{
					{Old: "old.txt", New: "new.txt"},
				},
			},
			wantErr: false,
		},
		{
			name:    "save entry with multiple operations",
			dirPath: "",
			entry: Entry{
				Version:   "1.0",
				Timestamp: time.Date(2024, 2, 20, 14, 0, 0, 0, time.UTC),
				Command:   "batch-rename",
				Operations: []Operation{
					{Old: "file1.txt", New: "renamed1.txt"},
					{Old: "file2.txt", New: "renamed2.txt"},
					{Old: "file3.txt", New: "renamed3.txt"},
				},
				Skipped: []Skipped{
					{Path: "skip.txt", Reason: "permission denied"},
				},
			},
			wantErr: false,
		},
		{
			name:    "save entry with collisions",
			dirPath: "",
			entry: Entry{
				Version:   "1.0",
				Timestamp: time.Date(2024, 3, 10, 8, 15, 30, 0, time.UTC),
				Command:   "rename",
				Collisions: []Collision{
					{Source1: "a.txt", Source2: "b.txt", Target: "c.txt"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathID := &mockPathIdentifier{ids: tt.pathIDs}
			store, tmpDir := newTestStore(t, pathID)

			dirPath := tt.dirPath
			if dirPath == "" {
				dirPath = tmpDir
			}

			fileName, err := store.Save(dirPath, tt.entry)

			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)
			assert.NotEqual(t, fileName, "")

			absPath, _ := filepath.Abs(dirPath)
			dirID, _ := pathID.PathIdentifier(absPath)
			histDir := filepath.Join(tmpDir, historySubDir, sanitizeDirID(dirID))

			filePath := filepath.Join(histDir, fileName)
			_, err = os.Stat(filePath)
			assert.Nil(t, err)

			data, err := os.ReadFile(filePath)
			assert.Nil(t, err)

			var saved Entry
			err = json.Unmarshal(data, &saved)
			assert.Nil(t, err)
			assert.Equal(t, saved.Version, tt.entry.Version)
			assert.Equal(t, saved.Command, tt.entry.Command)
			assert.Len(t, saved.Operations, len(tt.entry.Operations))
		})
	}
}

func TestGlobalStoreLatest(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, store *GlobalStore, tmpDir string) string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, entry *Entry)
	}{
		{
			name: "get latest from single entry",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				entry := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
					Command:   "single-entry",
					Operations: []Operation{
						{Old: "old.txt", New: "new.txt"},
					},
				}
				_, err := store.Save(tmpDir, entry)
				assert.Nil(t, err)
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, entry *Entry) {
				assert.Equal(t, entry.Command, "single-entry")
				assert.Len(t, entry.Operations, 1)
			},
		},
		{
			name: "get latest from multiple entries",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				entry1 := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Command:   "first-entry",
				}
				_, err := store.Save(tmpDir, entry1)
				assert.Nil(t, err)

				entry2 := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					Command:   "second-entry",
				}
				_, err = store.Save(tmpDir, entry2)
				assert.Nil(t, err)

				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, entry *Entry) {
				assert.Equal(t, entry.Command, "second-entry")
			},
		},
		{
			name: "no history returns error",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				absPath, _ := filepath.Abs(tmpDir)
				dirID, _ := store.pathID.PathIdentifier(absPath)
				histDir := filepath.Join(store.configDir, historySubDir, sanitizeDirID(dirID))
				os.MkdirAll(histDir, 0755)
				return tmpDir
			},
			wantErr: true,
		},
		{
			name: "operations sorted by depth",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				entry := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
					Command:   "depth-test",
					Operations: []Operation{
						{Old: "a/b/c/d", New: "1"},
						{Old: "a", New: "2"},
						{Old: "a/b/c", New: "3"},
						{Old: "a/b", New: "4"},
					},
				}
				_, err := store.Save(tmpDir, entry)
				assert.Nil(t, err)
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, entry *Entry) {
				assert.Equal(t, entry.Command, "depth-test")
				assert.Len(t, entry.Operations, 4)
				assert.Equal(t, entry.Operations[0].Old, "a")
				assert.Equal(t, entry.Operations[1].Old, "a/b")
				assert.Equal(t, entry.Operations[2].Old, "a/b/c")
				assert.Equal(t, entry.Operations[3].Old, "a/b/c/d")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathID := &mockPathIdentifier{ids: make(map[string]string)}
			store, tmpDir := newTestStore(t, pathID)

			dirPath := tt.setup(t, store, tmpDir)

			entry, err := store.Latest(dirPath)

			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)
			assert.NotNil(t, entry)

			if tt.validate != nil {
				tt.validate(t, entry)
			}
		})
	}
}

func TestGlobalStoreDelete(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, store *GlobalStore, tmpDir string) string
		wantErr  bool
		validate func(t *testing.T, store *GlobalStore, tmpDir, dirPath string)
	}{
		{
			name: "delete single entry",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				entry := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
					Command:   "to-delete",
				}
				_, err := store.Save(tmpDir, entry)
				assert.Nil(t, err)
				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, store *GlobalStore, tmpDir, dirPath string) {
				_, err := store.Latest(dirPath)
				assert.NotNil(t, err)
			},
		},
		{
			name: "delete latest keeps older entries",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				entry1 := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Command:   "older-entry",
				}
				_, err := store.Save(tmpDir, entry1)
				assert.Nil(t, err)

				entry2 := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					Command:   "newer-entry",
				}
				_, err = store.Save(tmpDir, entry2)
				assert.Nil(t, err)

				return tmpDir
			},
			wantErr: false,
			validate: func(t *testing.T, store *GlobalStore, tmpDir, dirPath string) {
				entry, err := store.Latest(dirPath)
				assert.Nil(t, err)
				assert.Equal(t, entry.Command, "older-entry")
			},
		},
		{
			name: "delete from empty history returns error",
			setup: func(t *testing.T, store *GlobalStore, tmpDir string) string {
				absPath, _ := filepath.Abs(tmpDir)
				dirID, _ := store.pathID.PathIdentifier(absPath)
				histDir := filepath.Join(store.configDir, historySubDir, sanitizeDirID(dirID))
				os.MkdirAll(histDir, 0755)
				return tmpDir
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathID := &mockPathIdentifier{ids: make(map[string]string)}
			store, tmpDir := newTestStore(t, pathID)

			dirPath := tt.setup(t, store, tmpDir)

			err := store.Delete(dirPath)

			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)

			if tt.validate != nil {
				tt.validate(t, store, tmpDir, dirPath)
			}
		})
	}
}

func TestGlobalStoreCleanup(t *testing.T) {
	tests := []struct {
		name          string
		numEntries    int
		expectedAfter int
	}{
		{
			name:          "no cleanup needed with 1 entry",
			numEntries:    1,
			expectedAfter: 1,
		},
		{
			name:          "no cleanup needed with 2 entries",
			numEntries:    2,
			expectedAfter: 2,
		},
		{
			name:          "cleanup removes oldest when 3 entries",
			numEntries:    3,
			expectedAfter: 2,
		},
		{
			name:          "cleanup removes oldest when 5 entries",
			numEntries:    5,
			expectedAfter: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathID := &mockPathIdentifier{ids: make(map[string]string)}
			store, tmpDir := newTestStore(t, pathID)

			for i := range tt.numEntries {
				entry := Entry{
					Version:   "1.0",
					Timestamp: time.Date(2024, 1, i+1, 0, 0, 0, 0, time.UTC),
					Command:   "test-entry",
				}
				_, err := store.Save(tmpDir, entry)
				assert.Nil(t, err)
			}

			absPath, _ := filepath.Abs(tmpDir)
			dirID, _ := pathID.PathIdentifier(absPath)
			histDir := filepath.Join(tmpDir, historySubDir, sanitizeDirID(dirID))

			entries, err := os.ReadDir(histDir)
			assert.Nil(t, err)

			jsonCount := 0
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
					jsonCount++
				}
			}

			assert.Equal(t, jsonCount, tt.expectedAfter)
		})
	}
}

func TestGlobalStoreDirHistoryPath(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		dirID     string
		expected  string
	}{
		{
			name:      "simple path",
			configDir: "/config",
			dirID:     "project1",
			expected:  filepath.Join("/config", historySubDir, "project1"),
		},
		{
			name:      "path with colons gets sanitized",
			configDir: "/config",
			dirID:     "C:/Users",
			expected:  filepath.Join("/config", historySubDir, "C_/Users"),
		},
		{
			name:      "empty dir ID",
			configDir: "/config",
			dirID:     "",
			expected:  filepath.Join("/config", historySubDir, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &GlobalStore{configDir: tt.configDir}
			result := store.dirHistoryPath(tt.dirID)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestGlobalStoreLoadEntry(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(t *testing.T, entry *Entry)
	}{
		{
			name: "valid entry",
			content: `{
				"version": "1.0",
				"timestamp": "2024-01-15T10:30:45Z",
				"path": "/test/path",
				"dir_id": "test-id",
				"command": "rename",
				"operations": [
					{"old": "a.txt", "new": "b.txt"}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, entry *Entry) {
				assert.Equal(t, entry.Version, "1.0")
				assert.Equal(t, entry.Command, "rename")
				assert.Len(t, entry.Operations, 1)
			},
		},
		{
			name: "entry with unsorted operations gets sorted",
			content: `{
				"version": "1.0",
				"timestamp": "2024-01-15T10:30:45Z",
				"command": "rename",
				"operations": [
					{"old": "a/b/c", "new": "1"},
					{"old": "a", "new": "2"},
					{"old": "a/b", "new": "3"}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, entry *Entry) {
				assert.Len(t, entry.Operations, 3)
				assert.Equal(t, entry.Operations[0].Old, "a")
				assert.Equal(t, entry.Operations[1].Old, "a/b")
				assert.Equal(t, entry.Operations[2].Old, "a/b/c")
			},
		},
		{
			name:    "invalid json",
			content: `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty content",
			content: ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := &GlobalStore{}

			filePath := filepath.Join(tmpDir, "test.json")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			assert.Nil(t, err)

			entry, err := store.loadEntry(filePath)

			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)
			assert.NotNil(t, entry)

			if tt.validate != nil {
				tt.validate(t, entry)
			}
		})
	}
}
