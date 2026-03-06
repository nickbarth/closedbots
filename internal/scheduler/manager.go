package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/runner"
	"github.com/nickbarth/closedbots/internal/store"
)

type EventType string

const (
	EventStatus   EventType = "status"
	EventStep     EventType = "step"
	EventSchedule EventType = "schedule"
)

type Event struct {
	Type               EventType
	TaskID             string
	Mode               domain.ScheduleMode
	State              string
	Message            string
	Error              string
	RunStatus          domain.RunStatus
	CurrentStepIndex   int
	CurrentStepID      string
	CurrentStepStatus  domain.StepStatus
	PreScreenshotPath  string
	PostScreenshotPath string
	NextRunAt          *time.Time
	CountdownSeconds   int
	LastRunStatus      domain.ScheduleRunStatus
}

type Manager struct {
	taskStore *store.TaskStore
	runner    *runner.Runner
	onEvent   func(Event)

	mu              sync.Mutex
	activeTaskID    string
	activeRunCancel context.CancelFunc
	scheduleTaskID  string
	scheduleCancel  context.CancelFunc
}

func NewManager(taskStore *store.TaskStore, run *runner.Runner, onEvent func(Event)) *Manager {
	return &Manager{
		taskStore: taskStore,
		runner:    run,
		onEvent:   onEvent,
	}
}

func (m *Manager) emit(e Event) {
	if m.onEvent != nil {
		m.onEvent(e)
	}
}

func (m *Manager) ActiveTaskID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeTaskID
}

func (m *Manager) RunNow(ctx context.Context, taskID string) error {
	result, err := m.runTask(ctx, taskID, "")
	if err != nil {
		m.emit(Event{Type: EventStatus, TaskID: taskID, State: "failed", Message: err.Error(), Error: err.Error(), RunStatus: domain.RunStatusFailed})
		return err
	}
	switch result.Status {
	case domain.RunStatusSuccess:
		m.emit(Event{Type: EventStatus, TaskID: taskID, State: "completed", Message: "run completed", RunStatus: domain.RunStatusSuccess})
		return nil
	case domain.RunStatusStopped:
		m.emit(Event{Type: EventStatus, TaskID: taskID, State: "stopped", Message: result.Error, RunStatus: domain.RunStatusStopped})
		return nil
	default:
		msg := result.Error
		if msg == "" {
			msg = "run failed"
		}
		m.emit(Event{Type: EventStatus, TaskID: taskID, State: "failed", Message: msg, Error: msg, RunStatus: domain.RunStatusFailed})
		return errors.New(msg)
	}
}

func (m *Manager) StartLoop(parent context.Context, taskID string, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	m.mu.Lock()
	if m.scheduleCancel != nil || m.activeTaskID != "" {
		m.mu.Unlock()
		return fmt.Errorf("another task is active; stop it first")
	}
	ctx, cancel := context.WithCancel(parent)
	m.scheduleCancel = cancel
	m.scheduleTaskID = taskID
	next := time.Now().UTC().Add(interval)
	m.mu.Unlock()

	go m.runScheduleLoop(ctx, taskID, interval)
	m.emit(Event{Type: EventSchedule, TaskID: taskID, Mode: domain.ScheduleLoop, State: "scheduled", NextRunAt: &next})
	return nil
}

func (m *Manager) StartCron(parent context.Context, taskID string, interval time.Duration) error {
	return m.StartLoop(parent, taskID, interval)
}

func (m *Manager) Stop(taskID, reason string) error {
	if reason == "" {
		reason = "stopped by user"
	}

	m.mu.Lock()
	target := taskID
	if target == "" {
		if m.activeTaskID != "" {
			target = m.activeTaskID
		} else if m.scheduleTaskID != "" {
			target = m.scheduleTaskID
		}
	}
	if target == "" {
		m.mu.Unlock()
		return nil
	}

	if m.activeRunCancel != nil && (m.activeTaskID == target || taskID == "") {
		m.activeRunCancel()
	}
	if m.scheduleCancel != nil && (m.scheduleTaskID == target || taskID == "") {
		m.scheduleCancel()
		m.scheduleCancel = nil
		m.scheduleTaskID = ""
	}
	m.mu.Unlock()

	m.emit(Event{Type: EventSchedule, TaskID: target, State: "stopped", Message: reason, LastRunStatus: domain.ScheduleRunStopped})
	return nil
}

func (m *Manager) runTask(parent context.Context, taskID string, mode domain.ScheduleMode) (domain.RunResult, error) {
	m.mu.Lock()
	if m.scheduleTaskID != "" {
		// Only schedule loop workers are allowed to execute while a schedule is active.
		if mode != domain.ScheduleLoop {
			m.mu.Unlock()
			return domain.RunResult{}, fmt.Errorf("another task is active; stop it first")
		}
		if m.scheduleTaskID != taskID {
			m.mu.Unlock()
			return domain.RunResult{}, fmt.Errorf("another task is active; stop it first")
		}
	}
	if m.activeTaskID != "" && m.activeTaskID != taskID {
		m.mu.Unlock()
		return domain.RunResult{}, fmt.Errorf("another task is active; stop it first")
	}
	if m.activeTaskID == taskID {
		m.mu.Unlock()
		return domain.RunResult{}, fmt.Errorf("task already running")
	}
	ctx, cancel := context.WithCancel(parent)
	m.activeTaskID = taskID
	m.activeRunCancel = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if m.activeRunCancel != nil {
			m.activeRunCancel = nil
		}
		m.activeTaskID = ""
		m.mu.Unlock()
	}()

	task, err := m.taskStore.Get(taskID)
	if err != nil {
		return domain.RunResult{}, err
	}
	m.emit(Event{Type: EventStatus, TaskID: taskID, Mode: mode, State: "running", Message: "run started", RunStatus: domain.RunStatusRunning})
	return m.runner.Run(ctx, task, func(update runner.StepUpdate) {
		m.emit(Event{
			Type:               EventStep,
			TaskID:             taskID,
			Mode:               mode,
			State:              "running",
			RunStatus:          update.RunStatus,
			CurrentStepIndex:   update.StepIndex,
			CurrentStepID:      update.StepID,
			CurrentStepStatus:  update.Status,
			Message:            update.Message,
			Error:              update.Error,
			PreScreenshotPath:  update.PreScreenshotPath,
			PostScreenshotPath: update.PostScreenshotPath,
		})
	})
}

func (m *Manager) runScheduleLoop(ctx context.Context, taskID string, interval time.Duration) {
	defer func() {
		m.mu.Lock()
		if m.scheduleTaskID == taskID {
			m.scheduleTaskID = ""
			m.scheduleCancel = nil
		}
		m.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, runErr := m.runTask(ctx, taskID, domain.ScheduleLoop)
		finish := time.Now().UTC()

		if runErr != nil {
			msg := runErr.Error()
			m.emit(Event{Type: EventStatus, TaskID: taskID, Mode: domain.ScheduleLoop, State: "failed", Message: msg, Error: msg, LastRunStatus: domain.ScheduleRunFailed, RunStatus: domain.RunStatusFailed})
			return
		}
		switch result.Status {
		case domain.RunStatusSuccess:
			next := finish.Add(interval)
			m.emit(Event{Type: EventStatus, TaskID: taskID, Mode: domain.ScheduleLoop, State: "completed", Message: "run completed", LastRunStatus: domain.ScheduleRunSuccess, RunStatus: domain.RunStatusSuccess})

			for {
				now := time.Now().UTC()
				if !now.Before(next) {
					break
				}
				remaining := int(next.Sub(now).Seconds())
				m.emit(Event{Type: EventSchedule, TaskID: taskID, Mode: domain.ScheduleLoop, State: "countdown", NextRunAt: &next, CountdownSeconds: remaining})
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
				}
			}
		case domain.RunStatusStopped:
			msg := result.Error
			if msg == "" {
				msg = "stopped by user"
			}
			m.emit(Event{Type: EventStatus, TaskID: taskID, Mode: domain.ScheduleLoop, State: "stopped", Message: msg, LastRunStatus: domain.ScheduleRunStopped, RunStatus: domain.RunStatusStopped})
			return
		default:
			msg := result.Error
			if msg == "" {
				msg = "run failed"
			}
			m.emit(Event{Type: EventStatus, TaskID: taskID, Mode: domain.ScheduleLoop, State: "failed", Message: msg, Error: msg, LastRunStatus: domain.ScheduleRunFailed, RunStatus: domain.RunStatusFailed})
			return
		}
	}
}

func CronPresetToDuration(preset string) (time.Duration, error) {
	switch preset {
	case "1m":
		return 1 * time.Minute, nil
	case "5m":
		return 5 * time.Minute, nil
	case "15m":
		return 15 * time.Minute, nil
	case "30m":
		return 30 * time.Minute, nil
	case "1h":
		return 1 * time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "12h":
		return 12 * time.Hour, nil
	case "24h", "daily":
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported preset %q", preset)
	}
}
