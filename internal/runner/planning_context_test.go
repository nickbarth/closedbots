package runner

import (
	"strings"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestBuildPlanningContext(t *testing.T) {
	task := domain.Task{
		Steps: []domain.Step{
			{ID: "s1", Instruction: "open", Status: domain.StepCompleted},
			{ID: "s2", Instruction: "verify", Status: domain.StepPending},
		},
	}
	ctx := buildPlanningContext(task, 1, map[string]string{"s1": "ok"})
	if !strings.Contains(ctx, "Completed steps:") || !strings.Contains(ctx, "Current step to execute next:") {
		t.Fatalf("ctx=%q", ctx)
	}
	if !strings.Contains(ctx, "Result: ok") {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestBuildPlanningContextNoCompletedAndBounds(t *testing.T) {
	task := domain.Task{
		Steps: []domain.Step{{ID: "s1", Instruction: "open", Status: domain.StepPending}},
	}
	ctx := buildPlanningContext(task, 9, nil)
	if !strings.Contains(ctx, "none yet.") {
		t.Fatalf("ctx=%q", ctx)
	}
	if strings.Contains(ctx, "[s2]") {
		t.Fatalf("unexpected context: %q", ctx)
	}
}

func TestFormatStepLine(t *testing.T) {
	got := formatStepLine(1, domain.Step{ID: "x", Instruction: "  hello "})
	if got != "2. [x] hello" {
		t.Fatalf("got=%q", got)
	}
}
