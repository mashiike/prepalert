package prepalert

import "strings"

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

func extructSection(memo string, header string) string {
	indexHeader := strings.Index(header, "# ")
	if indexHeader == -1 {
		return ""
	}
	atxHeaderPrefix := header[:indexHeader+2]
	index := strings.Index(memo, header)
	if index == -1 {
		return ""
	}
	sectionText := strings.TrimPrefix(memo[index:], header)
	index = strings.Index(sectionText, "\n"+atxHeaderPrefix)
	if index == -1 {
		return header + sectionText
	}
	return header + sectionText[:index]
}
