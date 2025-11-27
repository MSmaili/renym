package common

// MapSlice transforms a slice from type T to type U using the provided mapper function
func MapSlice[T, U any](slice []T, fn func(T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}
