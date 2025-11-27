package common

import (
	"strconv"
	"testing"
)

func TestMapSlice(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "converts integers to strings",
			test: func(t *testing.T) {
				input := []int{1, 2, 3, 4, 5}
				result := MapSlice(input, func(i int) string { return strconv.Itoa(i) })
				expected := []string{"1", "2", "3", "4", "5"}

				if len(result) != len(expected) {
					t.Errorf("expected length %d, got %d", len(expected), len(result))
				}
				for i := range result {
					if result[i] != expected[i] {
						t.Errorf("at index %d: expected %v, got %v", i, expected[i], result[i])
					}
				}
			},
		},
		{
			name: "empty slice returns empty slice",
			test: func(t *testing.T) {
				input := []int{}
				result := MapSlice(input, func(i int) string { return strconv.Itoa(i) })

				if len(result) != 0 {
					t.Errorf("expected empty slice, got length %d", len(result))
				}
			},
		},
		{
			name: "nil slice returns nil",
			test: func(t *testing.T) {
				var input []int
				result := MapSlice(input, func(i int) string { return strconv.Itoa(i) })

				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}
