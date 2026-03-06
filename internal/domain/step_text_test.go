package domain

import "testing"

func TestParsePointFormSteps(t *testing.T) {
	input := "1. Open browser\r\n2) Go to site\n- Type query\n* Press enter\n\n  "
	got := ParsePointFormSteps(input)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	if got[0] != "Open browser" || got[1] != "Go to site" || got[2] != "Type query" || got[3] != "Press enter" {
		t.Fatalf("unexpected parse result: %#v", got)
	}
}

func TestFormatPointFormStepsSkipsEmpty(t *testing.T) {
	steps := []Step{
		{Instruction: "First"},
		{Instruction: "   "},
		{Instruction: "Third"},
	}
	got := FormatPointFormSteps(steps)
	want := "1. First\n3. Third"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSetStepsFromInstructions(t *testing.T) {
	task := &Task{}
	SetStepsFromInstructions(task, []string{" one ", "", "two"})
	if len(task.Steps) != 2 {
		t.Fatalf("len = %d, want 2", len(task.Steps))
	}
	if task.Steps[0].ID != "step_001" || task.Steps[1].ID != "step_003" {
		t.Fatalf("unexpected ids: %#v", task.Steps)
	}
	if task.Steps[0].Status != StepPending || task.Steps[1].Status != StepPending {
		t.Fatalf("unexpected status: %#v", task.Steps)
	}
}

func TestSetStepsFromInstructionsNilTask(t *testing.T) {
	SetStepsFromInstructions(nil, []string{"x"})
}

func TestResetStepStatuses(t *testing.T) {
	task := &Task{
		Steps: []Step{
			{Status: StepCompleted, LastError: "x", Attempts: 3},
			{Status: StepFailed, LastError: "y", Attempts: 1},
		},
	}
	ResetStepStatuses(task)
	for i, s := range task.Steps {
		if s.Status != StepPending {
			t.Fatalf("step %d status=%q", i, s.Status)
		}
		if s.LastError != "" {
			t.Fatalf("step %d last_error=%q", i, s.LastError)
		}
		if s.Attempts != 0 {
			t.Fatalf("step %d attempts=%d", i, s.Attempts)
		}
	}
}
