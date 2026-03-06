package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/tasklog"
)

type CodexCLI struct {
	Bin     string
	Model   string
	Timeout time.Duration
	WorkDir string
	TaskLog *tasklog.Logger
}

func NewCodexCLI(workDir string) *CodexCLI {
	return &CodexCLI{
		Bin:     "codex",
		Model:   "",
		Timeout: 60 * time.Second,
		WorkDir: workDir,
		TaskLog: tasklog.New(tasklog.ResolvePath(workDir)),
	}
}

func (c *CodexCLI) logf(format string, args ...any) {
	if c == nil || c.TaskLog == nil {
		return
	}
	c.TaskLog.Log(fmt.Sprintf(format, args...))
}

func (c *CodexCLI) GenerateInitialSteps(ctx context.Context, taskSummary string, targetStepCount int) ([]domain.StepDraft, error) {
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

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 20,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"instruction": map[string]any{"type": "string", "minLength": 1},
					},
					"required":             []string{"instruction"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"steps"},
		"additionalProperties": false,
	}

	var resp struct {
		Steps []domain.StepDraft `json:"steps"`
	}
	if err := c.execJSON(ctx, prompt, nil, schema, &resp); err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

func (c *CodexCLI) PlanStepActions(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, priorContext string) ([]domain.Action, error) {
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
	if err := c.execJSON(ctx, prompt, []string{preScreenshotPath}, actionPlanningSchema(), &resp); err != nil {
		return nil, err
	}
	return resp.Actions, nil
}

func actionPlanningSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"actions": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 8,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":   map[string]any{"type": "string"},
						"reason": map[string]any{"type": []string{"string", "null"}},
						"x":      map[string]any{"type": []string{"integer", "null"}},
						"y":      map[string]any{"type": []string{"integer", "null"}},
						"button": map[string]any{"type": []string{"string", "null"}},
						"text":   map[string]any{"type": []string{"string", "null"}},
						"key":    map[string]any{"type": []string{"string", "null"}},
						"keys": map[string]any{
							"type":  []string{"array", "null"},
							"items": map[string]any{"type": "string"},
						},
						"ms":    map[string]any{"type": []string{"integer", "null"}},
						"dx":    map[string]any{"type": []string{"integer", "null"}},
						"dy":    map[string]any{"type": []string{"integer", "null"}},
						"mode":  map[string]any{"type": []string{"string", "null"}},
						"index": map[string]any{"type": []string{"integer", "null"}},
					},
					"required": []string{
						"type",
						"reason",
						"x",
						"y",
						"button",
						"text",
						"key",
						"keys",
						"ms",
						"dx",
						"dy",
						"mode",
						"index",
					},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"actions"},
		"additionalProperties": false,
	}
}

func (c *CodexCLI) VerifyStep(ctx context.Context, task domain.Task, step domain.Step, preScreenshotPath, postScreenshotPath string, executedActions []domain.Action) (domain.VerificationResult, error) {
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

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"completed": map[string]any{"type": "boolean"},
			"reason":    map[string]any{"type": "string"},
		},
		"required":             []string{"completed", "reason"},
		"additionalProperties": false,
	}

	var resp domain.VerificationResult
	if err := c.execJSON(ctx, prompt, []string{preScreenshotPath, postScreenshotPath}, schema, &resp); err != nil {
		return domain.VerificationResult{}, err
	}
	return resp, nil
}

func (c *CodexCLI) execJSON(ctx context.Context, prompt string, imagePaths []string, schema any, out any) error {
	if strings.TrimSpace(prompt) == "" {
		c.logf("codex request rejected: empty prompt")
		return errors.New("codex prompt cannot be empty")
	}
	c.logf("codex request model=%q images=%d prompt=%s", c.Model, len(imagePaths), logTextSnippet(prompt))
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	schemaFile, err := os.CreateTemp("", "closedbots-schema-*.json")
	if err != nil {
		return err
	}
	schemaPath := schemaFile.Name()
	defer os.Remove(schemaPath)
	defer schemaFile.Close()

	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	if _, err := schemaFile.Write(schemaBytes); err != nil {
		return err
	}

	outFile, err := os.CreateTemp("", "closedbots-codex-out-*.json")
	if err != nil {
		return err
	}
	outPath := outFile.Name()
	defer os.Remove(outPath)
	_ = outFile.Close()

	args := []string{"exec", "--skip-git-repo-check", "--color", "never", "--output-schema", schemaPath, "--output-last-message", outPath}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	for _, img := range imagePaths {
		if strings.TrimSpace(img) == "" {
			continue
		}
		args = append(args, "--image", img)
	}
	// Force prompt via stdin for robust compatibility across codex CLI prompt parsing behavior.
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	if c.WorkDir != "" {
		cmd.Dir = c.WorkDir
	}
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		c.logf("codex error: %v stderr=%s", err, logTextSnippet(stderr.String()))
		return fmt.Errorf("codex exec failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		c.logf("codex read output error: %v", err)
		return fmt.Errorf("read codex output: %w", err)
	}
	c.logf("codex response raw=%s", logTextSnippet(string(b)))
	if err := json.Unmarshal(b, out); err == nil {
		return nil
	}
	trimmed := extractJSONObject(string(b))
	if trimmed == "" {
		return errors.New("codex output is not valid json")
	}
	if err := json.Unmarshal([]byte(trimmed), out); err != nil {
		return fmt.Errorf("parse codex json: %w", err)
	}
	return nil
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

func (c *CodexCLI) Name() string {
	if c.Bin == "" {
		return "codex"
	}
	return filepath.Base(c.Bin)
}
