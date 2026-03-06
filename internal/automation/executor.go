//go:build !robotgo

package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
)

type FallbackExecutor struct{}

func NewExecutor() Executor {
	return &FallbackExecutor{}
}

func (f *FallbackExecutor) BackendName() string {
	return "fallback-noop"
}

func (f *FallbackExecutor) Execute(ctx context.Context, action domain.Action) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch action.Type {
	case domain.ActionWait:
		d := time.Duration(action.MS) * time.Millisecond
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	case domain.ActionSwitchTab:
		if action.Mode == "app_specific" && len(action.Keys) == 0 {
			return errors.New("switch_tab app_specific requires keys")
		}
		return nil
	case domain.ActionClick, domain.ActionDoubleClick, domain.ActionMoveMouse, domain.ActionTypeText,
		domain.ActionSendKey, domain.ActionHotkey, domain.ActionScroll:
		// Fallback executor keeps the app runnable on systems without native automation libs.
		// To perform real input events, build with a native backend.
		if action.Type == domain.ActionTypeText && strings.TrimSpace(action.Text) == "" {
			return fmt.Errorf("type_text received empty text")
		}
		return nil
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}
