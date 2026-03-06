package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nickbarth/closedbots/internal/domain"
)

const maxCurrentStepInstructionChars = 120

func buildCurrentStepLabel(step domain.Step, stepIndex, total int) string {
	idx := stepIndex + 1
	if idx < 1 {
		idx = 1
	}
	if total > 0 && idx > total {
		idx = total
	}
	instruction := strings.TrimSpace(step.Instruction)
	if instruction == "" {
		instruction = "(No instruction)"
	}
	instruction = clipInstructionForProgressLabel(instruction, maxCurrentStepInstructionChars)
	return "Current Step: [" + stepStatusText(step.Status) + "] " + strconv.Itoa(idx) + ". " + instruction
}

func buildEmptyCurrentStepLabel() string {
	return "Ready"
}

func buildStepProgressCounts(current, total int) string {
	if total < 0 {
		total = 0
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	return "Step " + strconv.Itoa(current) + " / " + strconv.Itoa(total)
}

func computeStepProgressValue(stepIndex, total int) int {
	if total <= 0 || stepIndex < 0 {
		return 0
	}
	v := stepIndex + 1
	if v > total {
		return total
	}
	return v
}

func resolveStepIndex(steps []domain.Step, current int) int {
	if len(steps) == 0 {
		return -1
	}
	if current >= 0 && current < len(steps) {
		return current
	}
	for i := range steps {
		if steps[i].Status == domain.StepRunning {
			return i
		}
	}
	for i := range steps {
		if steps[i].Status == domain.StepFailed {
			return i
		}
	}
	for i := range steps {
		if steps[i].Status == domain.StepPending {
			return i
		}
	}
	return len(steps) - 1
}

func allStepsCompleted(steps []domain.Step) bool {
	if len(steps) == 0 {
		return false
	}
	for i := range steps {
		if steps[i].Status != domain.StepCompleted {
			return false
		}
	}
	return true
}

func stepStatusText(status domain.StepStatus) string {
	switch status {
	case domain.StepRunning:
		return "In Progress"
	case domain.StepCompleted:
		return "Completed"
	case domain.StepFailed:
		return "Failed"
	default:
		return "Pending"
	}
}

func clipInstructionForProgressLabel(instruction string, maxChars int) string {
	if maxChars <= 0 {
		return instruction
	}
	if utf8.RuneCountInString(instruction) <= maxChars {
		return instruction
	}
	if maxChars <= 3 {
		return "..."
	}
	runes := []rune(instruction)
	return string(runes[:maxChars-3]) + "..."
}
