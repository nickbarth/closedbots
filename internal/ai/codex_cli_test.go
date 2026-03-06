package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestCodexHelpers(t *testing.T) {
	c := NewCodexCLI(t.TempDir())
	if got := c.Name(); got != "codex" {
		t.Fatalf("name=%q", got)
	}
	c.Bin = "/tmp/mycodex"
	if got := c.Name(); got != "mycodex" {
		t.Fatalf("name=%q", got)
	}
	if got := extractJSONObject("prefix {\"x\":1} suffix"); got != "{\"x\":1}" {
		t.Fatalf("extract=%q", got)
	}
	if got := extractJSONObject("nope"); got != "" {
		t.Fatalf("extract=%q", got)
	}
	if build := actionPlanningSchema(); build == nil {
		t.Fatalf("schema nil")
	}
}

func TestCodexExecJSONEmptyPrompt(t *testing.T) {
	c := NewCodexCLI(t.TempDir())
	err := c.execJSON(context.Background(), " ", nil, map[string]any{"type": "object"}, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("err=%v", err)
	}
}

func TestCodexGeneratePlanVerifySuccess(t *testing.T) {
	script := `#!/usr/bin/env bash
set -eu
out=""
for ((i=1;i<=$#;i++)); do
  arg="${!i}"
  if [[ "$arg" == "--output-last-message" ]]; then
    j=$((i+1))
    out="${!j}"
  fi
done
prompt="$(cat)"
if [[ "$prompt" == *'Output schema: {"steps"'* ]]; then
  printf '{"steps":[{"instruction":"Open calculator"}]}' >"$out"
elif [[ "$prompt" == *'Output schema: {"actions"'* ]]; then
  printf '{"actions":[{"type":"wait","reason":"ok","x":0,"y":0,"button":"left","text":"","key":"","keys":[],"ms":100,"dx":0,"dy":0,"mode":"","index":0}]}' >"$out"
else
  printf '{"completed":true,"reason":"ok"}' >"$out"
fi
`
	c := NewCodexCLI(t.TempDir())
	c.Bin = writeExecutable(t, "codex-shim.sh", script)

	steps, err := c.GenerateInitialSteps(context.Background(), "do stuff", 2)
	if err != nil {
		t.Fatalf("GenerateInitialSteps: %v", err)
	}
	if len(steps) != 1 || steps[0].Instruction == "" {
		t.Fatalf("steps=%#v", steps)
	}

	actions, err := c.PlanStepActions(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "go"}, "", "")
	if err != nil {
		t.Fatalf("PlanStepActions: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != domain.ActionWait {
		t.Fatalf("actions=%#v", actions)
	}

	verify, err := c.VerifyStep(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "verify"}, "", "", actions)
	if err != nil {
		t.Fatalf("VerifyStep: %v", err)
	}
	if !verify.Completed {
		t.Fatalf("verify=%#v", verify)
	}
}

func TestCodexExecJSONFailures(t *testing.T) {
	failScript := `#!/usr/bin/env bash
set -eu
echo "boom" >&2
exit 7
`
	c := NewCodexCLI(t.TempDir())
	c.Bin = writeExecutable(t, "codex-fail.sh", failScript)
	err := c.execJSON(context.Background(), "x", nil, map[string]any{"type": "object"}, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "codex exec failed") {
		t.Fatalf("err=%v", err)
	}

	invalidScript := `#!/usr/bin/env bash
set -eu
out=""
for ((i=1;i<=$#;i++)); do
  arg="${!i}"
  if [[ "$arg" == "--output-last-message" ]]; then
    j=$((i+1))
    out="${!j}"
  fi
done
printf 'not json' >"$out"
cat >/dev/null
`
	c.Bin = writeExecutable(t, "codex-invalid.sh", invalidScript)
	err = c.execJSON(context.Background(), "x", nil, map[string]any{"type": "object"}, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "not valid json") {
		t.Fatalf("err=%v", err)
	}
}

func TestCodexAdditionalBranches(t *testing.T) {
	var nilCodex *CodexCLI
	nilCodex.logf("ignored")

	c := NewCodexCLI(t.TempDir())
	c.TaskLog = nil
	c.logf("ignored")

	successScript := `#!/usr/bin/env bash
set -eu
out=""
for ((i=1;i<=$#;i++)); do
  arg="${!i}"
  if [[ "$arg" == "--output-last-message" ]]; then
    j=$((i+1))
    out="${!j}"
  fi
done
printf '{"steps":[{"instruction":"Open app"}]}' >"$out"
cat >/dev/null
`
	c.Bin = writeExecutable(t, "codex-success.sh", successScript)
	c.Model = "test-model"
	if _, err := c.GenerateInitialSteps(context.Background(), "summary", 0); err != nil {
		t.Fatalf("GenerateInitialSteps targetStepCount<=0: %v", err)
	}

	if err := c.execJSON(context.Background(), "x", nil, make(chan int), &struct{}{}); err == nil {
		t.Fatalf("expected schema marshal error")
	}

	parseErrScript := `#!/usr/bin/env bash
set -eu
out=""
for ((i=1;i<=$#;i++)); do
  arg="${!i}"
  if [[ "$arg" == "--output-last-message" ]]; then
    j=$((i+1))
    out="${!j}"
  fi
done
printf 'prefix {bad json} suffix' >"$out"
cat >/dev/null
`
	c.Bin = writeExecutable(t, "codex-parse-err.sh", parseErrScript)
	err := c.execJSON(context.Background(), "x", []string{"", "/tmp/a.png"}, map[string]any{"type": "object"}, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "parse codex json") {
		t.Fatalf("expected parse json error, got %v", err)
	}

	readErrScript := `#!/usr/bin/env bash
set -eu
out=""
for ((i=1;i<=$#;i++)); do
  arg="${!i}"
  if [[ "$arg" == "--output-last-message" ]]; then
    j=$((i+1))
    out="${!j}"
  fi
done
printf '{"steps":[{"instruction":"x"}]}' >"$out"
rm -f "$out"
cat >/dev/null
`
	c.Bin = writeExecutable(t, "codex-read-err.sh", readErrScript)
	err = c.execJSON(context.Background(), "x", nil, map[string]any{"type": "object"}, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "read codex output") {
		t.Fatalf("expected read output error, got %v", err)
	}

	failScript := `#!/usr/bin/env bash
set -eu
echo "boom" >&2
exit 9
`
	c.Bin = writeExecutable(t, "codex-wrap-fail.sh", failScript)
	if _, err := c.PlanStepActions(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", ""); err == nil {
		t.Fatalf("expected plan error")
	}
	if _, err := c.VerifyStep(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", "", nil); err == nil {
		t.Fatalf("expected verify error")
	}
}
