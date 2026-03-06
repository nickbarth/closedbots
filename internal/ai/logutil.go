package ai

import "strings"

func logTextSnippet(s string) string {
	const max = 4000
	trimmed := strings.TrimSpace(s)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "...(truncated)"
}
