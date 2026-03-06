//go:build !robotgo

package automation

import (
	"context"
	"testing"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestFallbackExecutorBasics(t *testing.T) {
	exec := NewExecutor()
	if exec.BackendName() != "fallback-noop" {
		t.Fatalf("backend = %q", exec.BackendName())
	}
}

func TestFallbackExecutorExecute(t *testing.T) {
	f := &FallbackExecutor{}
	if err := f.Execute(context.Background(), domain.Action{Type: domain.ActionClick, X: 1, Y: 1}); err != nil {
		t.Fatalf("click: %v", err)
	}
	if err := f.Execute(context.Background(), domain.Action{Type: domain.ActionSwitchTab, Mode: "app_specific"}); err == nil {
		t.Fatalf("expected error for missing keys")
	}
	if err := f.Execute(context.Background(), domain.Action{Type: domain.ActionTypeText, Text: "  "}); err == nil {
		t.Fatalf("expected empty text error")
	}
	if err := f.Execute(context.Background(), domain.Action{Type: domain.ActionType("bad")}); err == nil {
		t.Fatalf("expected unsupported action error")
	}
}

func TestFallbackExecutorWaitAndCancel(t *testing.T) {
	f := &FallbackExecutor{}
	if err := f.Execute(context.Background(), domain.Action{Type: domain.ActionWait, MS: 1}); err != nil {
		t.Fatalf("wait: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.Execute(ctx, domain.Action{Type: domain.ActionWait, MS: int((10 * time.Millisecond) / time.Millisecond)}); err == nil {
		t.Fatalf("expected canceled")
	}
}
