package runner

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/osctrl"
)

type providerFunc struct {
	generate func(context.Context, string, int) ([]domain.StepDraft, error)
	plan     func(context.Context, domain.Task, domain.Step, string, string) ([]domain.Action, error)
	verify   func(context.Context, domain.Task, domain.Step, string, string, []domain.Action) (domain.VerificationResult, error)
}

func (p providerFunc) GenerateInitialSteps(ctx context.Context, taskSummary string, targetStepCount int) ([]domain.StepDraft, error) {
	if p.generate != nil {
		return p.generate(ctx, taskSummary, targetStepCount)
	}
	return []domain.StepDraft{{Instruction: "x"}}, nil
}

func (p providerFunc) PlanStepActions(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, priorContext string) ([]domain.Action, error) {
	if p.plan != nil {
		return p.plan(ctx, task, step, preScreenshotPath, priorContext)
	}
	return []domain.Action{{Type: domain.ActionWait, MS: 100, Reason: "ok"}}, nil
}

func (p providerFunc) VerifyStep(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, postScreenshotPath string, executedActions []domain.Action) (domain.VerificationResult, error) {
	if p.verify != nil {
		return p.verify(ctx, task, step, preScreenshotPath, postScreenshotPath, executedActions)
	}
	return domain.VerificationResult{Completed: true, Reason: "ok"}, nil
}

type capturerFunc struct {
	fn func(path string) (image.Rectangle, error)
}

func (c capturerFunc) CaptureFullScreen(path string) (image.Rectangle, error) {
	if c.fn != nil {
		return c.fn(path)
	}
	return image.Rect(0, 0, 200, 200), nil
}

type execFunc struct {
	backend string
	fn      func(context.Context, domain.Action) error
}

func (e execFunc) BackendName() string { return e.backend }
func (e execFunc) Execute(ctx context.Context, a domain.Action) error {
	if e.fn != nil {
		return e.fn(ctx, a)
	}
	return nil
}

type noopDriver struct{}

func (d noopDriver) Name() string { return "noop" }
func (d noopDriver) StartGlobalStopHotkey(combo string, onPress func()) (osctrl.HotkeyHandle, error) {
	return nil, nil
}
func (d noopDriver) MinimizeMainWindow()                {}
func (d noopDriver) RestoreMainWindow()                 {}
func (d noopDriver) LaunchBrowser(url string) error     { return nil }
func (d noopDriver) OpenPath(path string) error         { return nil }

func oneStepTask(instr string) *domain.Task {
	return &domain.Task{
		ID:      "task1",
		Summary: "test",
		Steps:   []domain.Step{{ID: "step_001", Instruction: instr, Status: domain.StepPending}},
	}
}

func TestRunnerRunDependencyErrors(t *testing.T) {
	r := &Runner{}
	if _, err := r.Run(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected nil task error")
	}
	if _, err := r.Run(context.Background(), oneStepTask("x"), nil); err == nil {
		t.Fatalf("expected deps error")
	}
}

func TestRunnerRunFallbackExecutorFails(t *testing.T) {
	r := &Runner{
		Provider: providerFunc{},
		Capturer: capturerFunc{},
		Executor: execFunc{backend: "fallback-noop"},
		RunsDir:  t.TempDir(),
	}
	res, err := r.Run(context.Background(), oneStepTask("click button"), nil)
	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("status=%q", res.Status)
	}
}

func TestRunnerRunSuccessAndVerifyIncomplete(t *testing.T) {
	r := &Runner{
		Provider: providerFunc{},
		Capturer: capturerFunc{},
		Executor: execFunc{backend: "robotgo"},
		RunsDir:  t.TempDir(),
	}
	res, err := r.Run(context.Background(), oneStepTask("click button"), nil)
	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if res.Status != domain.RunStatusSuccess {
		t.Fatalf("status=%q", res.Status)
	}

	r.Provider = providerFunc{
		verify: func(context.Context, domain.Task, domain.Step, string, string, []domain.Action) (domain.VerificationResult, error) {
			return domain.VerificationResult{Completed: false, Reason: "not done"}, nil
		},
	}
	res, err = r.Run(context.Background(), oneStepTask("verify result"), nil)
	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if res.Status != domain.RunStatusFailed || res.Error != "not done" {
		t.Fatalf("res=%#v", res)
	}
}

func TestRunnerRunFailurePaths(t *testing.T) {
	base := &Runner{
		Provider: providerFunc{},
		Capturer: capturerFunc{},
		Executor: execFunc{backend: "robotgo"},
		Driver:   noopDriver{},
		RunsDir:  t.TempDir(),
	}

	// Screenshot failure.
	base.Capturer = capturerFunc{fn: func(path string) (image.Rectangle, error) {
		return image.Rectangle{}, errors.New("shot")
	}}
	res, _ := base.Run(context.Background(), oneStepTask("click"), nil)
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed on screenshot")
	}

	// Planning failure.
	base.Capturer = capturerFunc{}
	base.Provider = providerFunc{plan: func(context.Context, domain.Task, domain.Step, string, string) ([]domain.Action, error) {
		return nil, errors.New("plan")
	}}
	res, _ = base.Run(context.Background(), oneStepTask("click"), nil)
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed on planning")
	}

	// Validation failure.
	base.Provider = providerFunc{plan: func(context.Context, domain.Task, domain.Step, string, string) ([]domain.Action, error) {
		return []domain.Action{{Type: domain.ActionClick, X: 9999, Y: 9999, Button: "left"}}, nil
	}}
	res, _ = base.Run(context.Background(), oneStepTask("click"), nil)
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed on validation")
	}

	// Action execution failure.
	base.Provider = providerFunc{plan: func(context.Context, domain.Task, domain.Step, string, string) ([]domain.Action, error) {
		return []domain.Action{{Type: domain.ActionWait, MS: 100, Reason: "ok"}}, nil
	}}
	base.Executor = execFunc{backend: "robotgo", fn: func(context.Context, domain.Action) error { return errors.New("exec") }}
	res, _ = base.Run(context.Background(), oneStepTask("click"), nil)
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed on execute")
	}

	// Verification error.
	base.Executor = execFunc{backend: "robotgo"}
	base.Provider = providerFunc{
		plan: func(context.Context, domain.Task, domain.Step, string, string) ([]domain.Action, error) {
			return []domain.Action{{Type: domain.ActionWait, MS: 100, Reason: "ok"}}, nil
		},
		verify: func(context.Context, domain.Task, domain.Step, string, string, []domain.Action) (domain.VerificationResult, error) {
			return domain.VerificationResult{}, errors.New("verify")
		},
	}
	res, _ = base.Run(context.Background(), oneStepTask("verify output"), nil)
	if res.Status != domain.RunStatusFailed {
		t.Fatalf("expected failed on verify error")
	}
}

func TestRunnerRunContextCanceled(t *testing.T) {
	r := &Runner{
		Provider: providerFunc{},
		Capturer: capturerFunc{},
		Executor: execFunc{backend: "robotgo"},
		RunsDir:  t.TempDir(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := r.Run(ctx, oneStepTask("click"), nil)
	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if res.Status != domain.RunStatusStopped {
		t.Fatalf("status=%q", res.Status)
	}
}

func TestFailStep(t *testing.T) {
	s := &domain.Step{}
	failStep(s, "oops")
	if s.Status != domain.StepFailed || s.LastError != "oops" {
		t.Fatalf("step=%#v", s)
	}
}
