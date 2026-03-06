package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	numberedStepPattern = regexp.MustCompile(`^\s*\d+[\.\)]\s+`)
	bulletStepPattern   = regexp.MustCompile(`^\s*[-*]\s+`)
)

func ParsePointFormSteps(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		line = numberedStepPattern.ReplaceAllString(line, "")
		line = bulletStepPattern.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func FormatPointFormSteps(steps []Step) string {
	lines := make([]string, 0, len(steps))
	for i, step := range steps {
		instruction := strings.TrimSpace(step.Instruction)
		if instruction == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, instruction))
	}
	return strings.Join(lines, "\n")
}

func SetStepsFromInstructions(p *Task, instructions []string) {
	if p == nil {
		return
	}
	steps := make([]Step, 0, len(instructions))
	for i, instruction := range instructions {
		trimmed := strings.TrimSpace(instruction)
		if trimmed == "" {
			continue
		}
		steps = append(steps, Step{
			ID:          fmt.Sprintf("step_%03d", i+1),
			Instruction: trimmed,
			Status:      StepPending,
		})
	}
	p.Steps = steps
}
