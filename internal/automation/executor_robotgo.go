//go:build robotgo

package automation

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/nickbarth/closedbots/internal/domain"
)

type RobotGoExecutor struct{}

func NewExecutor() Executor {
	return &RobotGoExecutor{}
}

func (r *RobotGoExecutor) BackendName() string {
	return "robotgo"
}

func (r *RobotGoExecutor) Execute(ctx context.Context, action domain.Action) error {
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
	case domain.ActionMoveMouse:
		robotgo.Move(action.X, action.Y)
		return nil
	case domain.ActionClick:
		btn := action.Button
		if btn == "" {
			btn = "left"
		}
		robotgo.Move(action.X, action.Y)
		robotgo.Click(btn, false)
		return nil
	case domain.ActionDoubleClick:
		btn := action.Button
		if btn == "" {
			btn = "left"
		}
		robotgo.Move(action.X, action.Y)
		robotgo.Click(btn, true)
		return nil
	case domain.ActionTypeText:
		robotgo.TypeStr(action.Text)
		return nil
	case domain.ActionSendKey:
		key := normalizeKeyName(action.Key)
		if key == "" {
			return fmt.Errorf("send_key key empty")
		}
		robotgo.KeyTap(key)
		return nil
	case domain.ActionHotkey:
		keys := normalizeKeySequence(action.Keys)
		if len(keys) == 0 {
			return fmt.Errorf("hotkey keys empty")
		}
		if len(keys) == 1 {
			robotgo.KeyTap(keys[0])
			return nil
		}
		main := keys[len(keys)-1]
		mods := keys[:len(keys)-1]
		robotgo.KeyTap(main, stringsToInterfaces(mods)...)
		return nil
	case domain.ActionScroll:
		robotgo.Scroll(action.DY, action.DX)
		return nil
	case domain.ActionSwitchTab:
		return r.switchTab(action)
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

func (r *RobotGoExecutor) switchTab(action domain.Action) error {
	ctrlOrCmd := "ctrl"
	if runtime.GOOS == "darwin" {
		ctrlOrCmd = "cmd"
	}
	switch action.Mode {
	case "next":
		robotgo.KeyTap("tab", ctrlOrCmd)
	case "prev":
		robotgo.KeyTap("tab", ctrlOrCmd, "shift")
	case "index":
		if action.Index < 1 || action.Index > 9 {
			return fmt.Errorf("invalid tab index %d", action.Index)
		}
		robotgo.KeyTap(strconv.Itoa(action.Index), ctrlOrCmd)
	case "app_specific":
		keys := normalizeKeySequence(action.Keys)
		if len(keys) == 0 {
			return fmt.Errorf("app_specific keys empty")
		}
		if len(keys) == 1 {
			robotgo.KeyTap(keys[0])
		} else {
			main := keys[len(keys)-1]
			mods := keys[:len(keys)-1]
			robotgo.KeyTap(main, stringsToInterfaces(mods)...)
		}
	default:
		return fmt.Errorf("unsupported switch_tab mode %q", action.Mode)
	}
	return nil
}

func stringsToInterfaces(items []string) []interface{} {
	if len(items) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}
