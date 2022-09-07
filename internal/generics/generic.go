package generics

func Nullif[T comparable](t T, empty T) *T {
	if t == empty {
		return nil
	}
	return &t
}

func Ptr[T any](t T) *T {
	return &t
}
