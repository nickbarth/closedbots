//go:build linux

package hotkey

import "testing"

func TestToXKeybind(t *testing.T) {
	got := toXKeybind(Combo{Key: "k", Modifiers: []string{"ctrl", "shift", "alt", "cmd"}})
	want := "Control-Shift-Mod1-Mod4-k"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStartErrorsBeforeXBind(t *testing.T) {
	if _, err := Start("Ctrl+K", nil); err == nil {
		t.Fatalf("expected nil callback error")
	}
	if _, err := Start("bad", func() {}); err == nil {
		t.Fatalf("expected parse error")
	}
	if _, err := Start("Ctrl+K", func() {}); err == nil {
		t.Fatalf("expected x11 connection error in headless test env")
	}
}

func TestListenerStopNilSafe(t *testing.T) {
	var l *Listener
	l.Stop()
}
