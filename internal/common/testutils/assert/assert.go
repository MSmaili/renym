package assert

import "testing"

// Equal checks if two values are equal
func Equal[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// NotEqual checks if two values are not equal
func NotEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got == want {
		t.Errorf("got %v, want it to be different from %v", got, want)
	}
}

// True checks if a condition is true
func True(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("expected true: %s", msg)
	}
}

// False checks if a condition is false
func False(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Errorf("expected false: %s", msg)
	}
}

// Nil checks if a value is nil
func Nil(t *testing.T, value any) {
	t.Helper()
	if value != nil {
		t.Errorf("expected nil, got %v", value)
	}
}

// Empty Checks if length of array is 0
func Empty[T any](t *testing.T, value []T) {
	t.Helper()
	if len(value) != 0 {
		t.Errorf("expected 0, got %v", len(value))
	}
}

// NotNil checks if a value is not nil
func NotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Error("expected non-nil value, got nil")
	}
}

// Len checks if a slice/map/string has expected length
func Len[T any](t *testing.T, slice []T, expectedLen int) {
	t.Helper()
	if len(slice) != expectedLen {
		t.Errorf("expected length %d, got %d", expectedLen, len(slice))
	}
}

// SliceEqual checks if two slices are equal
func SliceEqual[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("slice length mismatch: got %d, want %d", len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("at index %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
