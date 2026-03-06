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

type ClaudeCLI struct {
	Bin     string
	Model   string
	Timeout time.Duration
	WorkDir string
	TaskLog *tasklog.Logger
}

func NewClaudeCLI(workDir string) *ClaudeCLI {
	return &ClaudeCLI{
		Bin:     "claude",
		Model:   "",
		Timeout: 60 * time.Second,
		WorkDir: workDir,
		TaskLog: tasklog.New(tasklog.ResolvePath(workDir)),
	}
}

func (c *ClaudeCLI) logf(format string, args ...any) {
	if c == nil || c.TaskLog == nil {
		return
	}
	c.TaskLog.Log(fmt.Sprintf(format, args...))
}

func (c *ClaudeCLI) GenerateInitialSteps(ctx context.Context, taskSummary string, targetStepCount int) ([]domain.StepDraft, error) {
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
	if err := c.execJSON(ctx, prompt, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

func (c *ClaudeCLI) PlanStepActions(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, priorContext string) ([]domain.Action, error) {
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
	if err := c.execJSON(ctx, prompt, []string{preScreenshotPath}, &resp); err != nil {
		return nil, err
	}
	return resp.Actions, nil
}

func (c *ClaudeCLI) VerifyStep(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, postScreenshotPath string, executedActions []domain.Action) (domain.VerificationResult, error) {
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
	if err := c.execJSON(ctx, prompt, []string{preScreenshotPath, postScreenshotPath}, &resp); err != nil {
		return domain.VerificationResult{}, err
	}
	return resp, nil
}

func (c *ClaudeCLI) execJSON(ctx context.Context, prompt string, imagePaths []string, out any) error {
	if strings.TrimSpace(prompt) == "" {
		c.logf("claude request rejected: empty prompt")
		return errors.New("claude prompt cannot be empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	promptWithImages := appendImagePathContext(prompt, imagePaths)
	c.logf("claude request model=%q images=%d prompt=%s", c.Model, len(imagePaths), logTextSnippet(promptWithImages))

	args := []string{"-p"}
	if strings.TrimSpace(c.Model) != "" {
		args = append(args, "--model", strings.TrimSpace(c.Model))
	}
	args = append(args, promptWithImages)

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	if strings.TrimSpace(c.WorkDir) != "" {
		cmd.Dir = c.WorkDir
	}
	raw, err := cmd.CombinedOutput()
	if err != nil {
		c.logf("claude error: %v output=%s", err, logTextSnippet(string(raw)))
		return fmt.Errorf("claude exec failed: %w (%s)", err, strings.TrimSpace(string(raw)))
	}
	c.logf("claude response raw=%s", logTextSnippet(string(raw)))

	if err := json.Unmarshal(raw, out); err == nil {
		return nil
	}
	trimmed := extractJSONObject(string(raw))
	if trimmed == "" {
		return errors.New("claude output is not valid json")
	}
	if err := json.Unmarshal([]byte(trimmed), out); err != nil {
		return fmt.Errorf("parse claude json: %w", err)
	}
	return nil
}

func appendImagePathContext(prompt string, imagePaths []string) string {
	trimmedPrompt := strings.TrimSpace(prompt)
	if len(imagePaths) == 0 {
		return trimmedPrompt
	}
	lines := []string{trimmedPrompt, "", "Screenshot paths for visual context:"}
	for _, p := range imagePaths {
		path := strings.TrimSpace(p)
		if path == "" {
			continue
		}
		lines = append(lines, "- "+path)
	}
	return strings.Join(lines, "\n")
}
