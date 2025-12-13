package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

type mockAdapter struct {
	caseSensitive bool
	sanitize      func(string) string
}

func (m *mockAdapter) IsCaseSensitive() bool {
	return m.caseSensitive
}

func (m *mockAdapter) SanitizeName(name string) string {
	if m.sanitize != nil {
		return m.sanitize(name)
	}
	return name
}

type mockMode struct {
	transform func(string) string
}

func (m mockMode) Transform(input string) string {
	return m.transform(input)
}

func TestPlan(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name              string
		mode              RenameMode
		caseSensitive     bool
		existingFiles     []string
		inputPaths        []string
		expectedOps       []RenameOp
		expectedSkipped   []SkippedFile
		expectedCollCount int
	}{
		{
			name:          "no_change_skip",
			mode:          mockMode{transform: func(s string) string { return s }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
			},
			expectedOps: []RenameOp{},
			expectedSkipped: []SkippedFile{
				{Path: filepath.Join(tempDir, "foo.txt"), Reason: "no change"},
			},
			expectedCollCount: 0,
		},
		{
			name:          "rename same file name different extension recursivly",
			mode:          mockMode{transform: func(s string) string { return strings.ToUpper(s) }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "/test/fooBar.md"),
				filepath.Join(tempDir, "/test2/fooBar.md"),
			},
			expectedOps: []RenameOp{
				{
					OldPath: filepath.Join(tempDir, "/test/fooBar.md"),
					NewPath: filepath.Join(tempDir, "/test/FOOBAR.md"),
				},
				{
					OldPath: filepath.Join(tempDir, "/test2/fooBar.md"),
					NewPath: filepath.Join(tempDir, "/test2/FOOBAR.md"),
				},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
		{
			name:          "rename same file name different extension",
			mode:          mockMode{transform: func(s string) string { return strings.ToUpper(s) }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "fooBar.md"),
				filepath.Join(tempDir, "fooBar.txt"),
			},
			expectedOps: []RenameOp{
				{
					OldPath: filepath.Join(tempDir, "fooBar.md"),
					NewPath: filepath.Join(tempDir, "FOOBAR.md"),
				},
				{
					OldPath: filepath.Join(tempDir, "fooBar.txt"),
					NewPath: filepath.Join(tempDir, "FOOBAR.txt"),
				},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
		{
			name:          "simple_rename",
			mode:          mockMode{transform: func(s string) string { return "bar" }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "foo.txt"), NewPath: filepath.Join(tempDir, "bar.txt")},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
		{
			name:          "batch_duplicate_collision",
			mode:          mockMode{transform: func(s string) string { return "same" }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
				filepath.Join(tempDir, "bar.txt"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "foo.txt"), NewPath: filepath.Join(tempDir, "same.txt")},
			},
			expectedSkipped: []SkippedFile{
				{Path: filepath.Join(tempDir, "bar.txt"), Reason: "duplicate target in batch"},
			},
			expectedCollCount: 1,
		},
		{
			name:          "disk_collision",
			mode:          mockMode{transform: func(s string) string { return "existing" }},
			caseSensitive: true,
			existingFiles: []string{
				filepath.Join(tempDir, "existing.txt"),
			},
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
			},
			expectedOps: []RenameOp{},
			expectedSkipped: []SkippedFile{
				{Path: filepath.Join(tempDir, "foo.txt"), Reason: "target already exists"},
			},
			expectedCollCount: 1,
		},
		{
			name: "file_being_renamed_away",
			mode: mockMode{transform: func(s string) string {
				if s == "foo" {
					return "bar"
				}
				if s == "existing" {
					return "moved"
				}
				return s
			}},
			caseSensitive: true,
			existingFiles: []string{
				filepath.Join(tempDir, "existing.txt"),
				filepath.Join(tempDir, "foo.txt"),
			},
			inputPaths: []string{
				filepath.Join(tempDir, "existing.txt"),
				filepath.Join(tempDir, "foo.txt"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "existing.txt"), NewPath: filepath.Join(tempDir, "moved.txt")},
				{OldPath: filepath.Join(tempDir, "foo.txt"), NewPath: filepath.Join(tempDir, "bar.txt")},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
		{
			name: "swap_files",
			mode: mockMode{transform: func(s string) string {
				if s == "foo" {
					return "bar"
				}
				if s == "bar" {
					return "foo"
				}
				return s
			}},
			caseSensitive: true,
			existingFiles: []string{
				filepath.Join(tempDir, "foo.txt"),
				filepath.Join(tempDir, "bar.txt"),
			},
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
				filepath.Join(tempDir, "bar.txt"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "foo.txt"), NewPath: filepath.Join(tempDir, "bar.txt")},
				{OldPath: filepath.Join(tempDir, "bar.txt"), NewPath: filepath.Join(tempDir, "foo.txt")},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
		{
			name:          "multiple_files_to_same_target",
			mode:          mockMode{transform: func(s string) string { return "target" }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "file1.txt"),
				filepath.Join(tempDir, "file2.txt"),
				filepath.Join(tempDir, "file3.txt"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "file1.txt"), NewPath: filepath.Join(tempDir, "target.txt")},
			},
			expectedSkipped: []SkippedFile{
				{Path: filepath.Join(tempDir, "file2.txt"), Reason: "duplicate target in batch"},
				{Path: filepath.Join(tempDir, "file3.txt"), Reason: "duplicate target in batch"},
			},
			expectedCollCount: 2,
		},
		{
			name:          "preserve_extension",
			mode:          mockMode{transform: func(s string) string { return "renamed" }},
			caseSensitive: true,
			inputPaths: []string{
				filepath.Join(tempDir, "foo.txt"),
				filepath.Join(tempDir, "bar.md"),
			},
			expectedOps: []RenameOp{
				{OldPath: filepath.Join(tempDir, "foo.txt"), NewPath: filepath.Join(tempDir, "renamed.txt")},
				{OldPath: filepath.Join(tempDir, "bar.md"), NewPath: filepath.Join(tempDir, "renamed.md")},
			},
			expectedSkipped:   []SkippedFile{},
			expectedCollCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, path := range tt.existingFiles {
				err := os.WriteFile(path, []byte("test"), 0644)
				if err != nil {
					t.Fatalf("failed to create test file %s: %v", path, err)
				}
				defer os.Remove(path)
			}

			adapter := &mockAdapter{caseSensitive: tt.caseSensitive}
			engine := NewEngine(tt.mode, adapter)

			result := engine.Plan(tt.inputPaths)

			assert.Len(t, result.Operations, len(tt.expectedOps))
			for i, expectedOp := range tt.expectedOps {
				if i < len(result.Operations) {
					assert.Equal(t, result.Operations[i].OldPath, expectedOp.OldPath)
					assert.Equal(t, result.Operations[i].NewPath, expectedOp.NewPath)
				}
			}

			assert.Len(t, result.Skipped, len(tt.expectedSkipped))
			for i, expectedSkipped := range tt.expectedSkipped {
				if i < len(result.Skipped) {
					assert.Equal(t, result.Skipped[i].Path, expectedSkipped.Path)
					assert.Equal(t, result.Skipped[i].Reason, expectedSkipped.Reason)
				}
			}

			assert.Len(t, result.Collisions, tt.expectedCollCount)
		})
	}
}

func TestComputeNewPath(t *testing.T) {
	tests := []struct {
		name     string
		mode     RenameMode
		sanitize func(string) string
		input    string
		expected string
	}{
		{
			name:     "simple_transformation",
			mode:     mockMode{transform: func(s string) string { return "new" }},
			input:    "/path/to/old.txt",
			expected: "/path/to/new.txt",
		},
		{
			name:     "preserve_directory",
			mode:     mockMode{transform: func(s string) string { return "renamed" }},
			input:    "/deep/nested/path/file.txt",
			expected: "/deep/nested/path/renamed.txt",
		},
		{
			name:     "preserve_extension",
			mode:     mockMode{transform: func(s string) string { return "base" }},
			input:    "/path/file.tar.gz",
			expected: "/path/base.gz",
		},
		{
			name:     "no_extension",
			mode:     mockMode{transform: func(s string) string { return "renamed" }},
			input:    "/path/filename",
			expected: "/path/renamed",
		},
		{
			name:     "sanitize_then_transform",
			mode:     mockMode{transform: func(s string) string { return strings.ToUpper(s) }},
			sanitize: func(s string) string { return strings.ReplaceAll(s, "(", "") },
			input:    "/path/file(1).txt",
			expected: "/path/FILE1).txt",
		},
		{
			name: "sanitize_removes_parentheses_before_transform",
			mode: mockMode{transform: func(s string) string { return s }},
			sanitize: func(s string) string {
				s = strings.ReplaceAll(s, "(", "")
				s = strings.ReplaceAll(s, ")", "")
				return s
			},
			input:    "/path/new file name (1) - Copy (1).txt",
			expected: "/path/new file name 1 - Copy 1.txt",
		},
		{
			name: "sanitize_and_transform_combined",
			mode: mockMode{transform: func(s string) string {
				return strings.ReplaceAll(s, " ", "_")
			}},
			sanitize: func(s string) string {
				s = strings.ReplaceAll(s, "(", "")
				s = strings.ReplaceAll(s, ")", "")
				return s
			},
			input:    "/path/new file name (1) - Copy (1).txt",
			expected: "/path/new_file_name_1_-_Copy_1.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &mockAdapter{caseSensitive: true, sanitize: tt.sanitize}
			engine := NewEngine(tt.mode, adapter)

			result := engine.computeNewPathPerSelectedMode(tt.input)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestCompareKey(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		caseSensitive bool
		expected      string
	}{
		{
			name:          "case_sensitive_unchanged",
			path:          "/Path/To/File.txt",
			caseSensitive: true,
			expected:      "/Path/To/File.txt",
		},
		{
			name:          "case_insensitive_lowercase",
			path:          "/Path/To/File.txt",
			caseSensitive: false,
			expected:      "/path/to/file.txt",
		},
		{
			name:          "case_insensitive_already_lower",
			path:          "/path/to/file.txt",
			caseSensitive: false,
			expected:      "/path/to/file.txt",
		},
		{
			name:          "case_sensitive_lowercase",
			path:          "/path/to/file.txt",
			caseSensitive: true,
			expected:      "/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareKey(tt.path, tt.caseSensitive)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestHasDiskCollision(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		newPath       string
		createFile    bool
		beingRenamed  map[string]bool
		caseSensitive bool
		expected      bool
	}{
		{
			name:          "file_exists_not_being_renamed",
			newPath:       filepath.Join(tempDir, "existing.txt"),
			createFile:    true,
			beingRenamed:  map[string]bool{},
			caseSensitive: true,
			expected:      true,
		},
		{
			name:       "file_exists_being_renamed",
			newPath:    filepath.Join(tempDir, "renamed.txt"),
			createFile: true,
			beingRenamed: map[string]bool{
				filepath.Join(tempDir, "renamed.txt"): true,
			},
			caseSensitive: true,
			expected:      false,
		},
		{
			name:          "file_does_not_exist",
			newPath:       filepath.Join(tempDir, "nonexistent.txt"),
			createFile:    false,
			beingRenamed:  map[string]bool{},
			caseSensitive: true,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				err := os.WriteFile(tt.newPath, []byte("test"), 0644)
				if err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				defer os.Remove(tt.newPath)
			}

			adapter := &mockAdapter{caseSensitive: tt.caseSensitive}
			engine := NewEngine(nil, adapter)

			compareKey := compareKey(tt.newPath, tt.caseSensitive)
			result := engine.hasDiskCollision(tt.newPath, compareKey, tt.beingRenamed)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestPathDepth(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{
			name:     "root",
			path:     "/",
			expected: 1,
		},
		{
			name:     "single_level",
			path:     "/path",
			expected: 1,
		},
		{
			name:     "two_levels",
			path:     "/path/to",
			expected: 2,
		},
		{
			name:     "deep_nested",
			path:     "/very/deep/nested/path/here",
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathDepth(tt.path)
			assert.Equal(t, result, tt.expected)
		})
	}
}

func TestSortPathsByDepth(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "already_sorted",
			input:    []string{"/a/b/c", "/a/b", "/a"},
			expected: []string{"/a/b/c", "/a/b", "/a"},
		},
		{
			name:     "reverse_order",
			input:    []string{"/a", "/a/b", "/a/b/c"},
			expected: []string{"/a/b/c", "/a/b", "/a"},
		},
		{
			name:     "mixed_order",
			input:    []string{"/a/b", "/a/b/c", "/a"},
			expected: []string{"/a/b/c", "/a/b", "/a"},
		},
		{
			name:     "same_depth_stable",
			input:    []string{"/a/x", "/a/y", "/a/z"},
			expected: []string{"/a/x", "/a/y", "/a/z"},
		},
		{
			name:     "mixed_depths",
			input:    []string{"/shallow", "/very/deep/nested/path", "/mid/level"},
			expected: []string{"/very/deep/nested/path", "/mid/level", "/shallow"},
		},
		{
			name:     "single_path",
			input:    []string{"/single"},
			expected: []string{"/single"},
		},
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "real_world_scenario",
			input: []string{
				"/project",
				"/project/src",
				"/project/src/utils",
				"/project/src/components",
				"/project/tests",
			},
			expected: []string{
				"/project/src/utils",
				"/project/src/components",
				"/project/src",
				"/project/tests",
				"/project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := mockMode{transform: func(s string) string { return "upper" }}
			adapter := &mockAdapter{caseSensitive: true}
			engine := NewEngine(mode, adapter)
			result := engine.SortPathsByDepth(tt.input)
			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				if i < len(result) {
					assert.Equal(t, result[i], expected)
				}
			}
		})
	}
}

// TODO: needs fix and adjustements, curently ordering is done inside command... which might be wrong thing
// because we are losing a sort testing
func TestPlanWithDirectoryOrdering(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		directories bool
		inputPaths  []string
		expectedOps []RenameOp
	}{
		{
			name:        "directories_false_preserves_order",
			directories: false,
			inputPaths: []string{
				filepath.Join(tempDir, "file1.txt"),
				filepath.Join(tempDir, "sub", "file2.txt"),
				filepath.Join(tempDir, "sub", "deep", "file3.txt"),
			},
			expectedOps: []RenameOp{
				{
					OldPath: filepath.Join(tempDir, "file1.txt"),
					NewPath: filepath.Join(tempDir, "upper.txt"),
				},
				{
					OldPath: filepath.Join(tempDir, "sub", "file2.txt"),
					NewPath: filepath.Join(tempDir, "sub", "upper.txt"),
				},
				{
					OldPath: filepath.Join(tempDir, "sub", "deep", "file3.txt"),
					NewPath: filepath.Join(tempDir, "sub", "deep", "upper.txt"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := mockMode{transform: func(s string) string { return "upper" }}
			adapter := &mockAdapter{caseSensitive: true}
			engine := NewEngine(mode, adapter)

			result := engine.Plan(tt.inputPaths)

			assert.Len(t, result.Operations, len(tt.expectedOps))
			for i, expectedOp := range tt.expectedOps {
				if i < len(result.Operations) {
					assert.Equal(t, result.Operations[i].OldPath, expectedOp.OldPath)
					assert.Equal(t, result.Operations[i].NewPath, expectedOp.NewPath)
				}
			}
		})
	}
}
