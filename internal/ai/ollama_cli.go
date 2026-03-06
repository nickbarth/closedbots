package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/tasklog"
)

type OllamaCLI struct {
	Bin     string
	Model   string
	Timeout time.Duration
	WorkDir string
	TaskLog *tasklog.Logger
}

func NewOllamaCLI(workDir string) *OllamaCLI {
	return &OllamaCLI{
		Bin:     "ollama",
		Model:   "",
		Timeout: 60 * time.Second,
		WorkDir: workDir,
		TaskLog: tasklog.New(tasklog.ResolvePath(workDir)),
	}
}

func (o *OllamaCLI) logf(format string, args ...any) {
	if o == nil || o.TaskLog == nil {
		return
	}
	o.TaskLog.Log(fmt.Sprintf(format, args...))
}

func (o *OllamaCLI) GenerateInitialSteps(ctx context.Context, taskSummary string, targetStepCount int) ([]domain.StepDraft, error) {
	if targetStepCount <= 0 {
		targetStepCount = 7
	}
	prompt := fmt.Sprintf(`You draft UI automation steps.
Return ONLY valid JSON.
Summary: %q
Create %d medium-grain steps.
Each step must include instruction.
Do not include secrets.
Output schema: {"steps":[{"instruction":"..."}]}`,
		taskSummary, targetStepCount)

	var resp struct {
		Steps []domain.StepDraft `json:"steps"`
	}
	if err := o.execJSON(ctx, prompt, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

func (o *OllamaCLI) PlanStepActions(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, priorContext string) ([]domain.Action, error) {
	taskJSON, _ := json.Marshal(task)
	stepJSON, _ := json.Marshal(step)
	prompt := fmt.Sprintf(`You are an automation action planner.
Return ONLY valid JSON.
Given task and current step, output 1-8 actions using ONLY these types:
click, double_click, move_mouse, type_text, send_key, hotkey, wait, scroll, switch_tab.
For switch_tab mode must be one of: next, prev, index, app_specific.
If mode is app_specific include keys array.
Prefer direct click/type actions for text fields; avoid modifier shortcuts like CTRL+A unless strictly necessary.
Task: %s
Step: %s
Execution context (completed steps + current step): %s
Output schema: {"actions":[...]}.
`, string(taskJSON), string(stepJSON), priorContext)

	var resp struct {
		Actions []domain.Action `json:"actions"`
	}
	if err := o.execJSON(ctx, prompt, []string{preScreenshotPath}, &resp); err != nil {
		return nil, err
	}
	return resp.Actions, nil
}

func (o *OllamaCLI) VerifyStep(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, postScreenshotPath string, executedActions []domain.Action) (domain.VerificationResult, error) {
	taskJSON, _ := json.Marshal(task)
	stepJSON, _ := json.Marshal(step)
	actionJSON, _ := json.Marshal(executedActions)
	prompt := fmt.Sprintf(`You are a step verification agent.
Return ONLY valid JSON.
Decide whether the step instruction was completed after executing actions.
Task: %s
Step: %s
ExecutedActions: %s
Output: {"completed": true|false, "reason": "short reason"}`,
		string(taskJSON), string(stepJSON), string(actionJSON))

	var resp domain.VerificationResult
	if err := o.execJSON(ctx, prompt, []string{preScreenshotPath, postScreenshotPath}, &resp); err != nil {
		return domain.VerificationResult{}, err
	}
	return resp, nil
}

func (o *OllamaCLI) execJSON(ctx context.Context, prompt string, imagePaths []string, out any) error {
	if strings.TrimSpace(prompt) == "" {
		o.logf("ollama request rejected: empty prompt")
		return errors.New("ollama prompt cannot be empty")
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	model, err := o.resolveModel(ctx)
	if err != nil {
		return err
	}

	promptWithImages := appendImagePathContext(prompt, imagePaths)
	o.logf("ollama request model=%q images=%d prompt=%s", model, len(imagePaths), logTextSnippet(promptWithImages))

	args := []string{"run", model, "--format", "json", "--hidethinking", promptWithImages}
	cmd := exec.CommandContext(ctx, o.Bin, args...)
	if strings.TrimSpace(o.WorkDir) != "" {
		cmd.Dir = o.WorkDir
	}

	raw, err := cmd.CombinedOutput()
	if err != nil {
		o.logf("ollama error: %v output=%s", err, logTextSnippet(string(raw)))
		return fmt.Errorf("ollama exec failed: %w (%s)", err, strings.TrimSpace(string(raw)))
	}
	o.logf("ollama response raw=%s", logTextSnippet(string(raw)))

	if err := json.Unmarshal(raw, out); err == nil {
		return nil
	}
	trimmed := extractJSONObject(string(raw))
	if trimmed == "" {
		return errors.New("ollama output is not valid json")
	}
	if err := json.Unmarshal([]byte(trimmed), out); err != nil {
		return fmt.Errorf("parse ollama json: %w", err)
	}
	return nil
}

func (o *OllamaCLI) resolveModel(ctx context.Context) (string, error) {
	explicit := strings.TrimSpace(o.Model)
	if explicit != "" {
		return explicit, nil
	}

	cmd := exec.CommandContext(ctx, o.Bin, "list")
	if strings.TrimSpace(o.WorkDir) != "" {
		cmd.Dir = o.WorkDir
	}
	raw, err := cmd.CombinedOutput()
	if err != nil {
		o.logf("ollama list error: %v output=%s", err, logTextSnippet(string(raw)))
		return "", fmt.Errorf("ollama list failed: %w (%s)", err, strings.TrimSpace(string(raw)))
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], "NAME") {
			continue
		}
		return fields[0], nil
	}
	return "", errors.New("no local ollama models available; run: ollama pull <model>")
}
