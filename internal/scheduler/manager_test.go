package scheduler

import (
	"context"
	"image"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nickbarth/closedbots/internal/automation"
	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/runner"
	"github.com/nickbarth/closedbots/internal/store"
)

type schedProvider struct{}

func (schedProvider) GenerateInitialSteps(ctx context.Context, taskSummary string, targetStepCount int) ([]domain.StepDraft, error) {
	return []domain.StepDraft{{Instruction: "x"}}, nil
}
func (schedProvider) PlanStepActions(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, priorContext string) ([]domain.Action, error) {
	return []domain.Action{{Type: domain.ActionWait, MS: 100, Reason: "ok"}}, nil
}
func (schedProvider) VerifyStep(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, postScreenshotPath string, executedActions []domain.Action) (domain.VerificationResult, error) {
	return domain.VerificationResult{Completed: true, Reason: "ok"}, nil
}

type schedCapturer struct{}

func (schedCapturer) CaptureFullScreen(path string) (image.Rectangle, error) {
	return image.Rect(0, 0, 100, 100), nil
}

type schedExecutor struct{}

func (schedExecutor) BackendName() string                                { return "robotgo" }
func (schedExecutor) Execute(ctx context.Context, a domain.Action) error { return nil }

type schedCancelableExecutor struct{}

func (schedCancelableExecutor) BackendName() string { return "robotgo" }
func (schedCancelableExecutor) Execute(ctx context.Context, a domain.Action) error {
	if a.Type == domain.ActionWait && a.MS > 0 {
		t := time.NewTimer(time.Duration(a.MS) * time.Millisecond)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func newTaskStoreWithTask(t *testing.T) *store.TaskStore {
	t.Helper()
	dir := t.TempDir()
	ts := store.NewTaskStore(dir)
	if err := ts.EnsureDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	task := &domain.Task{
		Summary: "Task A",
		Steps:   []domain.Step{{Instruction: "click something"}},
	}
	if err := ts.Save(task); err != nil {
		t.Fatalf("save: %v", err)
	}
	return ts
}

func newRunnerForScheduler(tmpRuns string, ts *store.TaskStore) *runner.Runner {
	return &runner.Runner{
		Provider: schedProvider{},
		Capturer: schedCapturer{},
		Executor: schedExecutor{},
		RunsDir:  filepath.Join(tmpRuns, "runs"),
		SaveTask: ts.Save,
	}
}

func TestManagerRunNowAndStopAndActiveTaskID(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), nil)

	if got := m.ActiveTaskID(); got != "" {
		t.Fatalf("active task=%q", got)
	}
	if err := m.RunNow(context.Background(), taskID); err != nil {
		t.Fatalf("run now: %v", err)
	}
	if err := m.Stop("", ""); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestManagerStartLoopAndEvents(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID

	var (
		mu     sync.Mutex
		events []Event
	)
	cb := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), cb)

	if err := m.StartLoop(context.Background(), taskID, 300*time.Millisecond); err != nil {
		t.Fatalf("start loop: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	_ = m.Stop(taskID, "stop")

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatalf("expected events")
	}
	foundCompleted := false
	for _, e := range events {
		if e.Type == EventStatus && e.State == "completed" {
			foundCompleted = true
			break
		}
	}
	if !foundCompleted {
		t.Fatalf("expected completed status event")
	}
}

func TestManagerStartLoopValidationAndCronPreset(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), nil)
	list, _ := ts.List()
	taskID := list[0].ID

	if err := m.StartLoop(context.Background(), taskID, 0); err == nil {
		t.Fatalf("expected invalid interval error")
	}
	if err := m.StartCron(context.Background(), taskID, -time.Second); err == nil {
		t.Fatalf("expected invalid cron interval error")
	}

	cases := map[string]time.Duration{
		"1m":    time.Minute,
		"5m":    5 * time.Minute,
		"15m":   15 * time.Minute,
		"30m":   30 * time.Minute,
		"1h":    time.Hour,
		"6h":    6 * time.Hour,
		"12h":   12 * time.Hour,
		"24h":   24 * time.Hour,
		"daily": 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := CronPresetToDuration(in)
		if err != nil || got != want {
			t.Fatalf("preset=%q got=%v err=%v", in, got, err)
		}
	}
	if _, err := CronPresetToDuration("bad"); err == nil {
		t.Fatalf("expected unsupported preset error")
	}
}

func TestManagerRunNowErrorStoppedAndFailedPaths(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID

	events := make([]Event, 0, 8)
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), func(e Event) {
		events = append(events, e)
	})

	// runTask/get error path
	if err := m.RunNow(context.Background(), "missing-task"); err == nil {
		t.Fatalf("expected missing task error")
	}

	// stopped path
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.RunNow(ctx, taskID); err != nil {
		t.Fatalf("canceled run now should return nil, got %v", err)
	}

	// failed (default) path from fallback backend
	failRunner := &runner.Runner{
		Provider: schedProvider{},
		Capturer: schedCapturer{},
		Executor: automation.NewExecutor(),
		RunsDir:  filepath.Join(t.TempDir(), "runs"),
		SaveTask: ts.Save,
	}
	mFail := NewManager(ts, failRunner, func(e Event) {
		events = append(events, e)
	})
	if err := mFail.RunNow(context.Background(), taskID); err == nil {
		t.Fatalf("expected failed run error")
	}

	if len(events) == 0 {
		t.Fatalf("expected emitted events")
	}
}

func TestManagerRunTaskConflictBranches(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), nil)

	m.scheduleTaskID = "other"
	if _, err := m.runTask(context.Background(), taskID, ""); err == nil {
		t.Fatalf("expected schedule conflict for non-loop run")
	}
	if _, err := m.runTask(context.Background(), taskID, domain.ScheduleLoop); err == nil {
		t.Fatalf("expected different scheduled task conflict")
	}

	m.scheduleTaskID = ""
	m.activeTaskID = "other"
	if _, err := m.runTask(context.Background(), taskID, ""); err == nil {
		t.Fatalf("expected active task conflict")
	}

	m.activeTaskID = taskID
	if _, err := m.runTask(context.Background(), taskID, ""); err == nil {
		t.Fatalf("expected already running error")
	}

	m.activeTaskID = ""
	if _, err := m.runTask(context.Background(), "missing", ""); err == nil {
		t.Fatalf("expected missing task get error")
	}
}

func TestManagerStartLoopConflictAndStopBranches(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID
	m := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), nil)

	m.activeTaskID = "busy"
	if err := m.StartLoop(context.Background(), taskID, time.Minute); err == nil {
		t.Fatalf("expected start conflict with active task")
	}
	m.activeTaskID = ""
	_, cancelBusy := context.WithCancel(context.Background())
	m.scheduleCancel = cancelBusy
	if err := m.StartLoop(context.Background(), taskID, time.Minute); err == nil {
		t.Fatalf("expected start conflict with active schedule")
	}
	m.scheduleCancel = nil

	if err := m.Stop("", ""); err != nil {
		t.Fatalf("stop with no target should succeed: %v", err)
	}

	activeCanceled := false
	m.activeTaskID = taskID
	m.activeRunCancel = func() { activeCanceled = true }
	if err := m.Stop("", ""); err != nil {
		t.Fatalf("stop active run: %v", err)
	}
	if !activeCanceled {
		t.Fatalf("expected active run cancel")
	}

	scheduleCanceled := false
	m.activeTaskID = ""
	m.activeRunCancel = nil
	m.scheduleTaskID = taskID
	m.scheduleCancel = func() { scheduleCanceled = true }
	if err := m.Stop("", ""); err != nil {
		t.Fatalf("stop schedule: %v", err)
	}
	if !scheduleCanceled {
		t.Fatalf("expected schedule cancel")
	}
	if m.scheduleTaskID != "" || m.scheduleCancel != nil {
		t.Fatalf("expected schedule state cleared")
	}
}

func TestManagerRunScheduleLoopErrorFailedAndStopped(t *testing.T) {
	ts := newTaskStoreWithTask(t)
	list, _ := ts.List()
	taskID := list[0].ID

	// runErr branch (task missing)
	errEvents := make([]Event, 0, 4)
	mErr := NewManager(ts, newRunnerForScheduler(t.TempDir(), ts), func(e Event) {
		errEvents = append(errEvents, e)
	})
	ctxErr, cancelErr := context.WithCancel(context.Background())
	mErr.runScheduleLoop(ctxErr, "missing-task", 50*time.Millisecond)
	cancelErr()

	// failed status branch
	failEvents := make([]Event, 0, 4)
	failRunner := &runner.Runner{
		Provider: schedProvider{},
		Capturer: schedCapturer{},
		Executor: automation.NewExecutor(),
		RunsDir:  filepath.Join(t.TempDir(), "runs"),
		SaveTask: ts.Save,
	}
	mFail := NewManager(ts, failRunner, func(e Event) {
		failEvents = append(failEvents, e)
	})
	ctxFail, cancelFail := context.WithCancel(context.Background())
	mFail.runScheduleLoop(ctxFail, taskID, 50*time.Millisecond)
	cancelFail()

	// stopped status branch via context cancellation during wait action execution
	stopEvents := make([]Event, 0, 8)
	stopRunner := &runner.Runner{
		Provider: schedProvider{},
		Capturer: schedCapturer{},
		Executor: schedCancelableExecutor{},
		RunsDir:  filepath.Join(t.TempDir(), "runs"),
		SaveTask: ts.Save,
	}
	mStop := NewManager(ts, stopRunner, func(e Event) {
		stopEvents = append(stopEvents, e)
	})
	ctxStop, cancelStop := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancelStop()
	}()
	mStop.runScheduleLoop(ctxStop, taskID, 200*time.Millisecond)

	hasState := func(events []Event, state string) bool {
		for _, e := range events {
			if e.Type == EventStatus && e.State == state {
				return true
			}
		}
		return false
	}
	if !hasState(errEvents, "failed") {
		t.Fatalf("expected failed status event for run error path")
	}
	if !hasState(failEvents, "failed") {
		t.Fatalf("expected failed status event for failed run path")
	}
	if !hasState(stopEvents, "stopped") {
		t.Fatalf("expected stopped status event for canceled run path")
	}
}
