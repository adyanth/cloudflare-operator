package pointer

// To returns pointer to value.
func To[T any](v T) *T {
	return &v
}
