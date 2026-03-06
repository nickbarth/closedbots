package runner

import (
	"regexp"
	"strings"
)

var explicitVerificationPattern = regexp.MustCompile(`(?i)\b(verify|check)\b`)
var verificationActionHintPattern = regexp.MustCompile(`(?i)\b(click|double[- ]?click|tap|press|type|enter|open|launch|navigate|select|choose|scroll|drag|move|switch|close|clear|write|fill|paste|copy|search|hit|hotkey|key)\b`)

func requiresExplicitVerification(instruction string) bool {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "make sure") {
		return true
	}
	return explicitVerificationPattern.MatchString(trimmed)
}

func shouldPlanStepActions(instruction string) bool {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return true
	}
	if !requiresExplicitVerification(trimmed) {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "go to ") {
		return true
	}
	if strings.Contains(lower, "checkbox") {
		return true
	}
	return verificationActionHintPattern.MatchString(lower)
}
