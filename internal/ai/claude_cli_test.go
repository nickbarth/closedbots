package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestAppendImagePathContext(t *testing.T) {
	if got := appendImagePathContext(" hello ", nil); got != "hello" {
		t.Fatalf("got %q", got)
	}
	got := appendImagePathContext("prompt", []string{" /tmp/a.png ", ""})
	if !strings.Contains(got, "Screenshot paths for visual context:") || !strings.Contains(got, "/tmp/a.png") {
		t.Fatalf("got %q", got)
	}
}

func TestClaudeExecJSONEmptyPrompt(t *testing.T) {
	c := NewClaudeCLI(t.TempDir())
	err := c.execJSON(context.Background(), " ", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("err=%v", err)
	}
}

func TestClaudeGeneratePlanVerifySuccess(t *testing.T) {
	script := `#!/usr/bin/env bash
set -eu
shift # -p
if [[ "${1:-}" == "--model" ]]; then shift 2; fi
prompt="${1:-}"
if [[ "$prompt" == *'Output schema: {"steps"'* ]]; then
  printf '{"steps":[{"instruction":"Open app"}]}'
elif [[ "$prompt" == *'Output schema: {"actions"'* ]]; then
  printf '{"actions":[{"type":"wait","reason":"ok","x":0,"y":0,"button":"left","text":"","key":"","keys":[],"ms":100,"dx":0,"dy":0,"mode":"","index":0}]}'
else
  printf '{"completed":true,"reason":"ok"}'
fi
`
	c := NewClaudeCLI(t.TempDir())
	c.Bin = writeExecutable(t, "claude-shim.sh", script)

	steps, err := c.GenerateInitialSteps(context.Background(), "do", 3)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps=%#v err=%v", steps, err)
	}
	actions, err := c.PlanStepActions(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "go"}, "", "")
	if err != nil || len(actions) != 1 {
		t.Fatalf("actions=%#v err=%v", actions, err)
	}
	v, err := c.VerifyStep(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "verify"}, "", "", actions)
	if err != nil || !v.Completed {
		t.Fatalf("verify=%#v err=%v", v, err)
	}
}

func TestClaudeExecJSONFallbackAndError(t *testing.T) {
	wrapped := `#!/usr/bin/env bash
set -eu
shift
echo 'prefix {"steps":[{"instruction":"x"}]} suffix'
`
	c := NewClaudeCLI(t.TempDir())
	c.Bin = writeExecutable(t, "claude-wrapped.sh", wrapped)
	var out struct {
		Steps []domain.StepDraft `json:"steps"`
	}
	if err := c.execJSON(context.Background(), "Output schema: {\"steps\":[{\"instruction\":\"...\"}]}", nil, &out); err != nil {
		t.Fatalf("wrapped parse err=%v", err)
	}
	if len(out.Steps) != 1 {
		t.Fatalf("out=%#v", out)
	}

	fail := `#!/usr/bin/env bash
set -eu
echo "boom" >&2
exit 9
`
	c.Bin = writeExecutable(t, "claude-fail.sh", fail)
	err := c.execJSON(context.Background(), "x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "claude exec failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestClaudeAdditionalBranches(t *testing.T) {
	var nilClaude *ClaudeCLI
	nilClaude.logf("ignored")

	c := NewClaudeCLI(t.TempDir())
	c.TaskLog = nil
	c.logf("ignored")

	successScript := `#!/usr/bin/env bash
set -eu
shift
if [[ "${1:-}" == "--model" ]]; then shift 2; fi
printf '{"steps":[{"instruction":"ok"}]}'
`
	c.Bin = writeExecutable(t, "claude-success.sh", successScript)
	c.Model = "custom-model"
	if _, err := c.GenerateInitialSteps(context.Background(), "x", 0); err != nil {
		t.Fatalf("GenerateInitialSteps targetStepCount<=0: %v", err)
	}

	parseErrScript := `#!/usr/bin/env bash
set -eu
shift
echo 'prefix {bad json} suffix'
`
	c.Bin = writeExecutable(t, "claude-parse-err.sh", parseErrScript)
	err := c.execJSON(context.Background(), "x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "parse claude json") {
		t.Fatalf("expected parse claude json error, got %v", err)
	}

	failScript := `#!/usr/bin/env bash
set -eu
echo "boom" >&2
exit 7
`
	c.Bin = writeExecutable(t, "claude-fail-wrap.sh", failScript)
	if _, err := c.PlanStepActions(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", ""); err == nil {
		t.Fatalf("expected plan error")
	}
	if _, err := c.VerifyStep(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", "", nil); err == nil {
		t.Fatalf("expected verify error")
	}
}
