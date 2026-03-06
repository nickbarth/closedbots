package validation

import (
	"image"
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestValidateActionsHappyPath(t *testing.T) {
	actions := []domain.Action{
		{Type: domain.ActionClick, X: 10, Y: 10, Button: "left"},
		{Type: domain.ActionDoubleClick, X: 20, Y: 20, Button: "right"},
		{Type: domain.ActionMoveMouse, X: 5, Y: 5},
		{Type: domain.ActionTypeText, Text: "abc"},
		{Type: domain.ActionSendKey, Key: "enter"},
		{Type: domain.ActionHotkey, Keys: []string{"ctrl", "s"}},
		{Type: domain.ActionWait, MS: 200},
		{Type: domain.ActionScroll, DY: 1},
	}
	if err := ValidateActions(actions, image.Rect(0, 0, 100, 100)); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateActionsSwitchTabModes(t *testing.T) {
	cases := []domain.Action{
		{Type: domain.ActionSwitchTab, Mode: "next"},
		{Type: domain.ActionSwitchTab, Mode: "prev"},
		{Type: domain.ActionSwitchTab, Mode: "index", Index: 2},
		{Type: domain.ActionSwitchTab, Mode: "app_specific", Keys: []string{"ctrl", "tab"}},
	}
	for _, c := range cases {
		if err := ValidateActions([]domain.Action{c}, image.Rectangle{}); err != nil {
			t.Fatalf("action %+v err=%v", c, err)
		}
	}
}

func TestValidateActionsErrors(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	cases := []domain.Action{
		{Type: domain.ActionClick, X: 500, Y: 500},
		{Type: domain.ActionClick, X: 1, Y: 1, Button: "bad"},
		{Type: domain.ActionTypeText, Text: "  "},
		{Type: domain.ActionSendKey, Key: " "},
		{Type: domain.ActionHotkey, Keys: nil},
		{Type: domain.ActionWait, MS: 10},
		{Type: domain.ActionWait, MS: 70000},
		{Type: domain.ActionScroll, DX: 0, DY: 0},
		{Type: domain.ActionSwitchTab, Mode: ""},
		{Type: domain.ActionSwitchTab, Mode: "bad"},
		{Type: domain.ActionSwitchTab, Mode: "index", Index: 99},
		{Type: domain.ActionSwitchTab, Mode: "app_specific", Keys: nil},
		{Type: domain.ActionType("unknown")},
	}
	for i, a := range cases {
		err := ValidateActions([]domain.Action{a}, bounds)
		if err == nil {
			t.Fatalf("case %d expected error", i)
		}
	}
	if err := ValidateActions(nil, bounds); err == nil {
		t.Fatalf("expected empty actions error")
	}
	tooMany := make([]domain.Action, maxActionsPerStep+1)
	for i := range tooMany {
		tooMany[i] = domain.Action{Type: domain.ActionWait, MS: 100}
	}
	if err := ValidateActions(tooMany, bounds); err == nil {
		t.Fatalf("expected too-many-actions error")
	}
}
