package prepalert

func ptr[T any](v T) *T {
	return &v
}

func triming(msg string, limit int, abbreviatedMessage string) string {
	n := len(abbreviatedMessage)
	if n >= limit {
		return abbreviatedMessage[0:limit]
	}
	return msg[0:limit-n] + abbreviatedMessage
}
