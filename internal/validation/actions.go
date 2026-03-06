package validation

import (
	"errors"
	"fmt"
	"image"
	"strings"

	"github.com/nickbarth/closedbots/internal/domain"
)

const maxActionsPerStep = 8

func ValidateActions(actions []domain.Action, screenBounds image.Rectangle) error {
	if len(actions) == 0 {
		return errors.New("no actions returned")
	}
	if len(actions) > maxActionsPerStep {
		return fmt.Errorf("too many actions: %d > %d", len(actions), maxActionsPerStep)
	}
	for i, a := range actions {
		if err := validateAction(a, screenBounds); err != nil {
			return fmt.Errorf("action %d invalid: %w", i, err)
		}
	}
	return nil
}

func validateAction(a domain.Action, screenBounds image.Rectangle) error {
	switch a.Type {
	case domain.ActionClick, domain.ActionDoubleClick, domain.ActionMoveMouse:
		if !screenBounds.Empty() {
			if a.X < screenBounds.Min.X || a.X > screenBounds.Max.X || a.Y < screenBounds.Min.Y || a.Y > screenBounds.Max.Y {
				return fmt.Errorf("coordinates out of bounds: (%d,%d)", a.X, a.Y)
			}
		}
		if a.Type != domain.ActionMoveMouse {
			if a.Button == "" {
				a.Button = "left"
			}
			if a.Button != "left" && a.Button != "right" && a.Button != "middle" {
				return fmt.Errorf("invalid button %q", a.Button)
			}
		}
	case domain.ActionTypeText:
		if strings.TrimSpace(a.Text) == "" {
			return errors.New("text empty")
		}
	case domain.ActionSendKey:
		if strings.TrimSpace(a.Key) == "" {
			return errors.New("key empty")
		}
	case domain.ActionHotkey:
		if len(a.Keys) == 0 {
			return errors.New("hotkey keys empty")
		}
	case domain.ActionWait:
		if a.MS < 50 || a.MS > 60000 {
			return fmt.Errorf("wait ms out of range: %d", a.MS)
		}
	case domain.ActionScroll:
		if a.DX == 0 && a.DY == 0 {
			return errors.New("scroll values empty")
		}
	case domain.ActionSwitchTab:
		if a.Mode == "" {
			return errors.New("switch_tab mode empty")
		}
		switch a.Mode {
		case "next", "prev", "index", "app_specific":
		default:
			return fmt.Errorf("unsupported switch_tab mode %q", a.Mode)
		}
		if a.Mode == "index" {
			if a.Index < 1 || a.Index > 9 {
				return fmt.Errorf("switch_tab index must be between 1 and 9")
			}
		}
		if a.Mode == "app_specific" && len(a.Keys) == 0 {
			return errors.New("switch_tab app_specific requires keys")
		}
	default:
		return fmt.Errorf("unsupported action type %q", a.Type)
	}
	return nil
}
