package runner

import (
	"fmt"
	"strings"

	"github.com/nickbarth/closedbots/internal/domain"
)

func buildPlanningContext(task domain.Task, currentIndex int, completionReasons map[string]string) string {
	var b strings.Builder
	b.WriteString("Completed steps:")

	completedCount := 0
	lastCompletedIdx := currentIndex - 1
	if lastCompletedIdx >= len(task.Steps) {
		lastCompletedIdx = len(task.Steps) - 1
	}
	for idx := 0; idx <= lastCompletedIdx; idx++ {
		if idx < 0 || idx >= len(task.Steps) {
			continue
		}
		step := task.Steps[idx]
		if step.Status != domain.StepCompleted {
			continue
		}
		completedCount++
		b.WriteString("\n")
		b.WriteString(formatStepLine(idx, step))
		if reason := strings.TrimSpace(completionReasons[step.ID]); reason != "" {
			b.WriteString("\nResult: ")
			b.WriteString(reason)
		}
	}
	if completedCount == 0 {
		b.WriteString(" none yet.")
	}

	b.WriteString("\nCurrent step to execute next:")
	if currentIndex >= 0 && currentIndex < len(task.Steps) {
		b.WriteString("\n")
		b.WriteString(formatStepLine(currentIndex, task.Steps[currentIndex]))
	}

	return strings.TrimSpace(b.String())
}

func formatStepLine(idx int, step domain.Step) string {
	return fmt.Sprintf("%d. [%s] %s", idx+1, step.ID, strings.TrimSpace(step.Instruction))
}
