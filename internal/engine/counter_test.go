package engine

import (
	"testing"

	"github.com/MSmaili/rnm/internal/common/testutils/assert"
)

func TestNewCounter(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		expected int
	}{
		{
			name:     "start_at_zero",
			start:    0,
			expected: 0,
		},
		{
			name:     "start_at_one",
			start:    1,
			expected: 1,
		},
		{
			name:     "start_at_negative",
			start:    -5,
			expected: -5,
		},
		{
			name:     "start_at_large_number",
			start:    1000,
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := NewCounter(tt.start)
			assert.Equal(t, counter.global, tt.expected)
			assert.Equal(t, len(counter.perDir), 0)
		})
	}
}

func TestCounterNextGlobal(t *testing.T) {
	tests := []struct {
		name           string
		start          int
		callCount      int
		expectedValues []int
	}{
		{
			name:           "single_call_from_zero",
			start:          0,
			callCount:      1,
			expectedValues: []int{0},
		},
		{
			name:           "multiple_calls_from_zero",
			start:          0,
			callCount:      5,
			expectedValues: []int{0, 1, 2, 3, 4},
		},
		{
			name:           "single_call_from_one",
			start:          1,
			callCount:      1,
			expectedValues: []int{1},
		},
		{
			name:           "multiple_calls_from_one",
			start:          1,
			callCount:      3,
			expectedValues: []int{1, 2, 3},
		},
		{
			name:           "start_from_negative",
			start:          -2,
			callCount:      5,
			expectedValues: []int{-2, -1, 0, 1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := NewCounter(tt.start)

			for i := 0; i < tt.callCount; i++ {
				result := counter.Next("")
				assert.Equal(t, result, tt.expectedValues[i])
			}
		})
	}
}

func TestCounterNextPerDir(t *testing.T) {
	tests := []struct {
		name           string
		start          int
		dirs           []string
		expectedValues []int
	}{
		{
			name:           "single_dir_single_call",
			start:          0,
			dirs:           []string{"/path/to/dir"},
			expectedValues: []int{1},
		},
		{
			name:           "single_dir_multiple_calls",
			start:          0,
			dirs:           []string{"/path/to/dir", "/path/to/dir", "/path/to/dir"},
			expectedValues: []int{1, 2, 3},
		},
		{
			name:  "multiple_dirs_alternating",
			start: 0,
			dirs: []string{
				"/dir1",
				"/dir2",
				"/dir1",
				"/dir2",
				"/dir1",
			},
			expectedValues: []int{1, 1, 2, 2, 3},
		},
		{
			name:  "three_different_dirs",
			start: 0,
			dirs: []string{
				"/a",
				"/b",
				"/c",
				"/a",
				"/b",
				"/c",
			},
			expectedValues: []int{1, 1, 1, 2, 2, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := NewCounter(tt.start)

			for i, dir := range tt.dirs {
				result := counter.Next(dir)
				assert.Equal(t, result, tt.expectedValues[i])
			}
		})
	}
}

func TestCounterNextMixed(t *testing.T) {
	t.Run("global_and_per_dir_independent", func(t *testing.T) {
		counter := NewCounter(0)

		// Global calls
		assert.Equal(t, counter.Next(""), 0)
		assert.Equal(t, counter.Next(""), 1)

		// Per-dir calls should be independent
		assert.Equal(t, counter.Next("/dir1"), 1)
		assert.Equal(t, counter.Next("/dir1"), 2)

		// Global should continue from where it left off
		assert.Equal(t, counter.Next(""), 2)

		// New dir starts at 1
		assert.Equal(t, counter.Next("/dir2"), 1)

		// Existing dir continues
		assert.Equal(t, counter.Next("/dir1"), 3)
	})

	t.Run("global_start_does_not_affect_per_dir", func(t *testing.T) {
		counter := NewCounter(100)

		// Global starts at 100
		assert.Equal(t, counter.Next(""), 100)

		// Per-dir always starts at 1
		assert.Equal(t, counter.Next("/mydir"), 1)
		assert.Equal(t, counter.Next("/mydir"), 2)

		// Global continues
		assert.Equal(t, counter.Next(""), 101)
	})
}

func TestCounterPerDirIsolation(t *testing.T) {
	t.Run("different_dirs_are_isolated", func(t *testing.T) {
		counter := NewCounter(0)

		// Each directory should have its own counter starting at 1
		dirs := []string{"/a", "/b", "/c", "/d", "/e"}
		for _, dir := range dirs {
			result := counter.Next(dir)
			assert.Equal(t, result, 1)
		}

		// Second pass - each should return 2
		for _, dir := range dirs {
			result := counter.Next(dir)
			assert.Equal(t, result, 2)
		}
	})

	t.Run("similar_dir_names_are_distinct", func(t *testing.T) {
		counter := NewCounter(0)

		// These should all be treated as different directories
		assert.Equal(t, counter.Next("/path"), 1)
		assert.Equal(t, counter.Next("/path/sub"), 1)
		assert.Equal(t, counter.Next("/path/sub/deep"), 1)

		// Each increments independently
		assert.Equal(t, counter.Next("/path"), 2)
		assert.Equal(t, counter.Next("/path/sub"), 2)
		assert.Equal(t, counter.Next("/path/sub/deep"), 2)
	})
}

func TestCounterEdgeCases(t *testing.T) {
	t.Run("empty_string_uses_global", func(t *testing.T) {
		counter := NewCounter(5)

		// Empty string should use global counter
		assert.Equal(t, counter.Next(""), 5)
		assert.Equal(t, counter.Next(""), 6)
	})

	t.Run("whitespace_dir_treated_as_dir", func(t *testing.T) {
		counter := NewCounter(0)

		// Non-empty string (even whitespace) uses per-dir counter
		assert.Equal(t, counter.Next(" "), 1)
		assert.Equal(t, counter.Next(" "), 2)

		// Different whitespace is a different dir
		assert.Equal(t, counter.Next("  "), 1)
	})

	t.Run("special_characters_in_dir", func(t *testing.T) {
		counter := NewCounter(0)

		assert.Equal(t, counter.Next("/path/with spaces"), 1)
		assert.Equal(t, counter.Next("/path/with-dashes"), 1)
		assert.Equal(t, counter.Next("/path/with.dots"), 1)
		assert.Equal(t, counter.Next("/path/with(parens)"), 1)

		// All should increment independently
		assert.Equal(t, counter.Next("/path/with spaces"), 2)
		assert.Equal(t, counter.Next("/path/with-dashes"), 2)
	})
}
