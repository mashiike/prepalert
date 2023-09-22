package prepalert

func ptr[T any](v T) *T {
	return &v
}

func triming(msg string, limit int, abbreviatedMessage string) string {
	if len(msg) <= limit {
		return msg
	}
	n := len(abbreviatedMessage)
	if n >= limit {
		return abbreviatedMessage[0:limit]
	}
	return msg[0:limit-n] + abbreviatedMessage
}
