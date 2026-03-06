//go:build linux

package osctrl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppWindowTitle(t *testing.T) {
	if AppWindowTitle() != "Closed Bots" {
		t.Fatalf("unexpected title %q", AppWindowTitle())
	}
}

func TestNewDriverName(t *testing.T) {
	d := NewDriver()
	if d == nil {
		t.Fatalf("driver is nil")
	}
	if got := d.Name(); got == "" {
		t.Fatalf("empty name")
	}
}

func TestRunLinuxForegroundActivationErrors(t *testing.T) {
	if err := runLinuxForegroundActivation(""); err == nil {
		t.Fatalf("expected empty title error")
	}
	if err := runLinuxForegroundActivation("Definitely Not A Real Window Title"); err == nil {
		t.Fatalf("expected activation error")
	}
}

func TestLinuxDriverHotkeyParseError(t *testing.T) {
	d := &linuxDriver{}
	if _, err := d.StartGlobalStopHotkey("bad", func() {}); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestLinuxDriverLaunchBrowserAndOpenPath(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// chrome path preferred
	write("google-chrome", "#!/usr/bin/env bash\nexit 0\n")
	prevPath := os.Getenv("PATH")
	t.Setenv("PATH", dir)
	d := &linuxDriver{}
	if err := d.LaunchBrowser("https://example.com"); err != nil {
		t.Fatalf("launch browser via chrome: %v", err)
	}

	// fallback to xdg-open when chrome missing
	_ = os.Remove(filepath.Join(dir, "google-chrome"))
	write("xdg-open", "#!/usr/bin/env bash\nexit 0\n")
	if err := d.LaunchBrowser("https://example.com"); err != nil {
		t.Fatalf("launch browser fallback: %v", err)
	}
	if err := d.OpenPath("https://example.com"); err != nil {
		t.Fatalf("open path: %v", err)
	}

	// missing xdg-open should error
	t.Setenv("PATH", "")
	if err := d.OpenPath("https://example.com"); err == nil {
		t.Fatalf("expected open path error")
	}
	t.Setenv("PATH", prevPath)
}
