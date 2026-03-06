package automation

import "testing"

func TestNormalizeKeyName(t *testing.T) {
	if got := normalizeKeyName("  RETURN "); got != "enter" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeKeyName("x"); got != "x" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeKeyName(" "); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeKeySequence(t *testing.T) {
	got := normalizeKeySequence([]string{"Control", " ", "RETURN"})
	if len(got) != 2 || got[0] != "ctrl" || got[1] != "enter" {
		t.Fatalf("got %#v", got)
	}
	if normalizeKeySequence(nil) != nil {
		t.Fatalf("expected nil")
	}
}
