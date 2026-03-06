package domain

import (
	"time"
)

const (
	SchemaVersionTask     = 1
	SchemaVersionSchedule = 1
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
)

type ActionType string

const (
	ActionClick       ActionType = "click"
	ActionDoubleClick ActionType = "double_click"
	ActionMoveMouse   ActionType = "move_mouse"
	ActionTypeText    ActionType = "type_text"
	ActionSendKey     ActionType = "send_key"
	ActionHotkey      ActionType = "hotkey"
	ActionWait        ActionType = "wait"
	ActionScroll      ActionType = "scroll"
	ActionSwitchTab   ActionType = "switch_tab"
)

type Task struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	Summary       string    `json:"summary,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Steps         []Step    `json:"steps"`
}

type Step struct {
	ID          string     `json:"id"`
	Instruction string     `json:"instruction"`
	Status      StepStatus `json:"status"`
	LastError   string     `json:"last_error"`
	Attempts    int        `json:"attempts"`
}

type StepDraft struct {
	Instruction string `json:"instruction"`
}

type Action struct {
	Type   ActionType `json:"type"`
	Reason string     `json:"reason,omitempty"`

	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Button string `json:"button,omitempty"`

	Text string   `json:"text,omitempty"`
	Key  string   `json:"key,omitempty"`
	Keys []string `json:"keys,omitempty"`

	MS int `json:"ms,omitempty"`

	DX int `json:"dx,omitempty"`
	DY int `json:"dy,omitempty"`

	Mode  string `json:"mode,omitempty"`
	Index int    `json:"index,omitempty"`
}

type VerificationResult struct {
	Completed bool   `json:"completed"`
	Reason    string `json:"reason"`
}

type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
	RunStatusRunning RunStatus = "running"
	RunStatusStopped RunStatus = "stopped"
)

type RunResult struct {
	RunID         string    `json:"run_id"`
	TaskID        string    `json:"task_id"`
	Status        RunStatus `json:"status"`
	FailedStepID  string    `json:"failed_step_id,omitempty"`
	Error         string    `json:"error,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	CompletedStep int       `json:"completed_step"`
}

type ScheduleMode string

const (
	ScheduleLoop ScheduleMode = "loop"
	ScheduleCron ScheduleMode = "cron"
)

type ScheduleRunStatus string

const (
	ScheduleRunNone    ScheduleRunStatus = "none"
	ScheduleRunSuccess ScheduleRunStatus = "success"
	ScheduleRunFailed  ScheduleRunStatus = "failed"
	ScheduleRunStopped ScheduleRunStatus = "stopped"
)

type Schedule struct {
	TaskID         string            `json:"task_id"`
	Mode           ScheduleMode      `json:"mode"`
	Interval       string            `json:"interval"`
	Enabled        bool              `json:"enabled"`
	LastRunStatus  ScheduleRunStatus `json:"last_run_status"`
	LastStartedAt  *time.Time        `json:"last_started_at,omitempty"`
	LastFinishedAt *time.Time        `json:"last_finished_at,omitempty"`
	NextRunAt      *time.Time        `json:"next_run_at,omitempty"`
	DisabledReason string            `json:"disabled_reason,omitempty"`
}

type ScheduleFile struct {
	SchemaVersion int        `json:"schema_version"`
	Schedules     []Schedule `json:"schedules"`
}

type RunLogEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	RunID     string    `json:"run_id"`
	StepID    string    `json:"step_id,omitempty"`
	Message   string    `json:"message"`
}
