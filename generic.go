package prepalert

func nullif[T comparable](t T, empty T) *T {
	if t == empty {
		return nil
	}
	return &t
}
