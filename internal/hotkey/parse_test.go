package hotkey

import "testing"

func TestParseCombo(t *testing.T) {
	c, err := ParseCombo("Ctrl+Shift+K")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Key != "k" {
		t.Fatalf("key=%q", c.Key)
	}
	if len(c.Modifiers) != 2 || c.Modifiers[0] != "ctrl" || c.Modifiers[1] != "shift" {
		t.Fatalf("mods=%#v", c.Modifiers)
	}
}

func TestParseComboErrors(t *testing.T) {
	for _, in := range []string{"", "k", "ctrl+", "weird+z"} {
		if _, err := ParseCombo(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestNormalizeModifier(t *testing.T) {
	cases := map[string]string{
		"control": "ctrl",
		"shift":   "shift",
		"alt":     "alt",
		"meta":    "cmd",
	}
	for in, want := range cases {
		got, ok := normalizeModifier(in)
		if !ok || got != want {
			t.Fatalf("in=%q got=%q ok=%v", in, got, ok)
		}
	}
	if _, ok := normalizeModifier("bad"); ok {
		t.Fatalf("expected false")
	}
}
