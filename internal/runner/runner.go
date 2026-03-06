package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nickbarth/closedbots/internal/ai"
	"github.com/nickbarth/closedbots/internal/automation"
	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/osctrl"
	"github.com/nickbarth/closedbots/internal/validation"
)

type StepUpdate struct {
	RunID              string
	TaskID             string
	StepIndex          int
	StepID             string
	Status             domain.StepStatus
	RunStatus          domain.RunStatus
	Message            string
	Error              string
	PreScreenshotPath  string
	PostScreenshotPath string
	StartedAt          time.Time
	FinishedAt         time.Time
}

type Runner struct {
	Provider ai.Provider
	Capturer automation.Capturer
	Executor automation.Executor
	Driver   osctrl.Driver
	RunsDir  string
	SaveTask func(*domain.Task) error
}

func (r *Runner) Run(ctx context.Context, task *domain.Task, onUpdate func(StepUpdate)) (domain.RunResult, error) {
	if task == nil {
		return domain.RunResult{}, fmt.Errorf("task is nil")
	}
	if r.Provider == nil || r.Capturer == nil || r.Executor == nil {
		return domain.RunResult{}, fmt.Errorf("runner dependencies are not fully configured")
	}
	if err := os.MkdirAll(r.RunsDir, 0o755); err != nil {
		return domain.RunResult{}, err
	}

	runID := "run_" + uuid.NewString()[:8]
	started := time.Now().UTC()
	result := domain.RunResult{
		RunID:     runID,
		TaskID:    task.ID,
		Status:    domain.RunStatusRunning,
		StartedAt: started,
	}

	runDir := filepath.Join(r.RunsDir, fmt.Sprintf("%s_%s", started.Format("20060102_150405"), task.ID))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return result, err
	}
	logPath := filepath.Join(runDir, "run.jsonl")
	logFile, err := os.Create(logPath)
	if err != nil {
		return result, err
	}
	defer logFile.Close()
	logWriter := bufio.NewWriter(logFile)
	defer logWriter.Flush()

	logEvent := func(level, stepID, msg string) {
		e := domain.RunLogEvent{
			Timestamp: time.Now().UTC(),
			Level:     level,
			RunID:     runID,
			StepID:    stepID,
			Message:   msg,
		}
		b, _ := json.Marshal(e)
		_, _ = logWriter.Write(append(b, '\n'))
	}

	completionReasons := make(map[string]string)
	fallbackExecutor := strings.TrimSpace(r.Executor.BackendName()) == "fallback-noop"
	for i := range task.Steps {
		select {
		case <-ctx.Done():
			result.Status = domain.RunStatusStopped
			result.Error = "stopped by user"
			result.FinishedAt = time.Now().UTC()
			return result, nil
		default:
		}

		step := &task.Steps[i]
		prePath := filepath.Join(runDir, fmt.Sprintf("step_%03d_pre.png", i+1))
		postPath := ""

		finalize := func(status domain.RunStatus, msg string, updateStatus domain.StepStatus) (domain.RunResult, error) {
			if status == domain.RunStatusFailed {
				failStep(step, msg)
				result.FailedStepID = step.ID
				result.Error = msg
				result.Status = domain.RunStatusFailed
			} else if status == domain.RunStatusStopped {
				step.Status = updateStatus
				step.LastError = msg
				result.FailedStepID = step.ID
				result.Error = msg
				result.Status = domain.RunStatusStopped
			}
			result.FinishedAt = time.Now().UTC()
			if r.SaveTask != nil {
				_ = r.SaveTask(task)
			}
			level := "error"
			if status == domain.RunStatusStopped {
				level = "warn"
			}
			logEvent(level, step.ID, msg)
			if onUpdate != nil {
				onUpdate(StepUpdate{
					RunID:              runID,
					TaskID:             task.ID,
					StepIndex:          i,
					StepID:             step.ID,
					Status:             step.Status,
					RunStatus:          status,
					Message:            msg,
					Error:              msg,
					PreScreenshotPath:  prePath,
					PostScreenshotPath: postPath,
					FinishedAt:         result.FinishedAt,
				})
			}
			return result, nil
		}
		if fallbackExecutor {
			return finalize(
				domain.RunStatusFailed,
				"native input backend unavailable; rebuild closedbots with -tags robotgo",
				domain.StepFailed,
			)
		}

		captureFailedPost := func() {
			if postPath != "" {
				return
			}
			failPath := filepath.Join(runDir, fmt.Sprintf("step_%03d_failed_post.png", i+1))
			if _, err := r.Capturer.CaptureFullScreen(failPath); err == nil {
				postPath = failPath
			}
		}

		step.Status = domain.StepRunning
		step.Attempts++
		step.LastError = ""
		if r.SaveTask != nil {
			_ = r.SaveTask(task)
		}
		if onUpdate != nil {
			onUpdate(StepUpdate{
				RunID:              runID,
				TaskID:             task.ID,
				StepIndex:          i,
				StepID:             step.ID,
				Status:             step.Status,
				RunStatus:          domain.RunStatusRunning,
				Message:            "running",
				PreScreenshotPath:  prePath,
				PostScreenshotPath: postPath,
				StartedAt:          time.Now().UTC(),
			})
		}
		logEvent("info", step.ID, "step started")

		screenBounds, err := r.Capturer.CaptureFullScreen(prePath)
		if err != nil {
			return finalize(domain.RunStatusFailed, fmt.Sprintf("screenshot failed: %v", err), domain.StepFailed)
		}

		actions, handledDeterministic, err := r.executeDeterministicStep(ctx, *task, *step)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
			}
			captureFailedPost()
			return finalize(domain.RunStatusFailed, fmt.Sprintf("deterministic action failed: %v", err), domain.StepFailed)
		}
		if !handledDeterministic {
			if shouldPlanStepActions(step.Instruction) {
				planningContext := buildPlanningContext(*task, i, completionReasons)
				actions, err = r.Provider.PlanStepActions(ctx, *task, *step, prePath, planningContext)
				if err != nil {
					if errors.Is(err, context.Canceled) || ctx.Err() != nil {
						return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
					}
					captureFailedPost()
					return finalize(domain.RunStatusFailed, fmt.Sprintf("action planning failed: %v", err), domain.StepFailed)
				}
				if ctx.Err() != nil {
					return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
				}

				if err := validation.ValidateActions(actions, screenBounds); err != nil {
					captureFailedPost()
					return finalize(domain.RunStatusFailed, fmt.Sprintf("action validation failed: %v", err), domain.StepFailed)
				}

				for _, action := range actions {
					if err := r.Executor.Execute(ctx, action); err != nil {
						if errors.Is(err, context.Canceled) || ctx.Err() != nil {
							return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
						}
						captureFailedPost()
						return finalize(domain.RunStatusFailed, fmt.Sprintf("action execution failed: %v", err), domain.StepFailed)
					}
					if ctx.Err() != nil {
						return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
					}
				}
			}
		}

		if shouldSettleBeforePostCapture(actions) {
			if err := waitWithContext(ctx, time.Duration(postActionSettleMs)*time.Millisecond); err != nil {
				return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
			}
		}

		postPath = filepath.Join(runDir, fmt.Sprintf("step_%03d_post.png", i+1))
		if _, err := r.Capturer.CaptureFullScreen(postPath); err != nil {
			return finalize(domain.RunStatusFailed, fmt.Sprintf("post screenshot failed: %v", err), domain.StepFailed)
		}
		if ctx.Err() != nil {
			return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
		}

		verify := domain.VerificationResult{}
		if requiresExplicitVerification(step.Instruction) {
			verify, err = r.Provider.VerifyStep(ctx, *task, *step, prePath, postPath, actions)
			if err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					return finalize(domain.RunStatusStopped, "stopped by user", domain.StepFailed)
				}
				return finalize(domain.RunStatusFailed, fmt.Sprintf("verification failed: %v", err), domain.StepFailed)
			}
		} else {
			verify = domain.VerificationResult{
				Completed: true,
				Reason:    "No explicit verification requested; continuing.",
			}
		}

		if !verify.Completed {
			reason := strings.TrimSpace(verify.Reason)
			if reason == "" {
				reason = "step verification returned incomplete"
			}
			return finalize(domain.RunStatusFailed, reason, domain.StepFailed)
		}

		step.Status = domain.StepCompleted
		step.LastError = ""
		result.CompletedStep = i + 1
		if r.SaveTask != nil {
			_ = r.SaveTask(task)
		}
		if onUpdate != nil {
			onUpdate(StepUpdate{
				RunID:              runID,
				TaskID:             task.ID,
				StepIndex:          i,
				StepID:             step.ID,
				Status:             step.Status,
				RunStatus:          domain.RunStatusRunning,
				Message:            verify.Reason,
				PreScreenshotPath:  prePath,
				PostScreenshotPath: postPath,
				FinishedAt:         time.Now().UTC(),
			})
		}
		logEvent("info", step.ID, "step completed")
		completionReasons[step.ID] = strings.TrimSpace(verify.Reason)
	}

	result.Status = domain.RunStatusSuccess
	result.FinishedAt = time.Now().UTC()
	logEvent("info", "", "run completed")
	return result, nil
}

func failStep(step *domain.Step, msg string) {
	step.Status = domain.StepFailed
	step.LastError = msg
}
