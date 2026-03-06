package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestOllamaExecJSONEmptyPrompt(t *testing.T) {
	o := NewOllamaCLI(t.TempDir())
	err := o.execJSON(context.Background(), " ", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("err=%v", err)
	}
}

func TestOllamaGeneratePlanVerifySuccess(t *testing.T) {
	script := `#!/usr/bin/env bash
set -eu
cmd="${1:-}"
if [[ "$cmd" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
if [[ "$cmd" == "run" ]]; then
  prompt="${@: -1}"
  if [[ "$prompt" == *'Output schema: {"steps"'* ]]; then
    printf '{"steps":[{"instruction":"Open app"}]}'
  elif [[ "$prompt" == *'Output schema: {"actions"'* ]]; then
    printf '{"actions":[{"type":"wait","reason":"ok","x":0,"y":0,"button":"left","text":"","key":"","keys":[],"ms":100,"dx":0,"dy":0,"mode":"","index":0}]}'
  else
    printf '{"completed":true,"reason":"ok"}'
  fi
  exit 0
fi
echo "unknown command" >&2
exit 2
`
	o := NewOllamaCLI(t.TempDir())
	o.Bin = writeExecutable(t, "ollama-shim.sh", script)

	steps, err := o.GenerateInitialSteps(context.Background(), "do", 3)
	if err != nil || len(steps) != 1 {
		t.Fatalf("steps=%#v err=%v", steps, err)
	}
	actions, err := o.PlanStepActions(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "go"}, "", "")
	if err != nil || len(actions) != 1 {
		t.Fatalf("actions=%#v err=%v", actions, err)
	}
	v, err := o.VerifyStep(context.Background(), domain.Task{ID: "t1"}, domain.Step{ID: "s1", Instruction: "verify"}, "", "", actions)
	if err != nil || !v.Completed {
		t.Fatalf("verify=%#v err=%v", v, err)
	}
}

func TestOllamaResolveModelCases(t *testing.T) {
	o := NewOllamaCLI(t.TempDir())
	o.Model = "explicit-model"
	model, err := o.resolveModel(context.Background())
	if err != nil || model != "explicit-model" {
		t.Fatalf("model=%q err=%v", model, err)
	}

	emptyList := `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  exit 0
fi
exit 1
`
	o.Model = ""
	o.Bin = writeExecutable(t, "ollama-empty.sh", emptyList)
	_, err = o.resolveModel(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no local ollama models available") {
		t.Fatalf("err=%v", err)
	}

	failList := `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "boom" >&2
  exit 3
fi
exit 1
`
	o.Bin = writeExecutable(t, "ollama-fail.sh", failList)
	_, err = o.resolveModel(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ollama list failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestOllamaExecJSONErrorAndFallbackParse(t *testing.T) {
	wrapped := `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
if [[ "${1:-}" == "run" ]]; then
  echo 'prefix {"completed":true,"reason":"ok"} suffix'
  exit 0
fi
exit 1
`
	o := NewOllamaCLI(t.TempDir())
	o.Bin = writeExecutable(t, "ollama-wrap.sh", wrapped)
	var out domain.VerificationResult
	if err := o.execJSON(context.Background(), "verify", nil, &out); err != nil {
		t.Fatalf("err=%v", err)
	}
	if !out.Completed {
		t.Fatalf("out=%#v", out)
	}

	failRun := `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
echo "run failed" >&2
exit 4
`
	o.Bin = writeExecutable(t, "ollama-run-fail.sh", failRun)
	err := o.execJSON(context.Background(), "x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "ollama exec failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestOllamaAdditionalBranches(t *testing.T) {
	var nilOllama *OllamaCLI
	nilOllama.logf("ignored")

	o := NewOllamaCLI(t.TempDir())
	o.TaskLog = nil
	o.logf("ignored")

	successScript := `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
if [[ "${1:-}" == "run" ]]; then
  printf '{"steps":[{"instruction":"ok"}]}'
  exit 0
fi
exit 1
`
	o.Bin = writeExecutable(t, "ollama-success-extra.sh", successScript)
	if _, err := o.GenerateInitialSteps(context.Background(), "x", 0); err != nil {
		t.Fatalf("GenerateInitialSteps targetStepCount<=0: %v", err)
	}

	o.Bin = writeExecutable(t, "ollama-parse-err.sh", `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
echo 'prefix {bad json} suffix'
`)
	err := o.execJSON(context.Background(), "x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "parse ollama json") {
		t.Fatalf("expected parse ollama json error, got %v", err)
	}

	o.Bin = writeExecutable(t, "ollama-list-fail-extra.sh", `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "nope" >&2
  exit 3
fi
exit 1
`)
	err = o.execJSON(context.Background(), "x", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "ollama list failed") {
		t.Fatalf("expected resolve model error, got %v", err)
	}

	o.Bin = writeExecutable(t, "ollama-run-fail-extra.sh", `#!/usr/bin/env bash
set -eu
if [[ "${1:-}" == "list" ]]; then
  echo "NAME SIZE MODIFIED"
  echo "llama3 1GB now"
  exit 0
fi
echo "run failed" >&2
exit 4
`)
	if _, err := o.PlanStepActions(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", ""); err == nil {
		t.Fatalf("expected plan error")
	}
	if _, err := o.VerifyStep(context.Background(), domain.Task{}, domain.Step{Instruction: "x"}, "", "", nil); err == nil {
		t.Fatalf("expected verify error")
	}
}
