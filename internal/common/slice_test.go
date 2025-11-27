package common

import (
	"strconv"
	"testing"

	"github.com/MSmaili/rnm/internal/common/assert"
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

				assert.SliceEqual(t, result, expected)
			},
		},
		{
			name: "nil slice returns nil",
			test: func(t *testing.T) {
				var input []int
				result := MapSlice(input, func(i int) string { return strconv.Itoa(i) })

				assert.Empty(t, result)
			},
		},
		{
			name: "empty slice returns empty slice",
			test: func(t *testing.T) {
				input := []int{}
				result := MapSlice(input, func(i int) string { return strconv.Itoa(i) })

				assert.Len(t, result, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}
