package ui

import (
	"fmt"
	"strings"
)

func buildFriendlyErrorMessage(userMessage, logPath string) string {
	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		msg = "Something went wrong."
	}
	path := strings.TrimSpace(logPath)
	if path == "" {
		return msg + "\n\nTechnical details were written to task.log and it will be opened for you."
	}
	return fmt.Sprintf("%s\n\nTechnical details were written to:\n%s\n\nThe log file will now be opened for you.", msg, path)
}
