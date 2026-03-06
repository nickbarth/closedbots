//go:build !linux && !windows && !darwin

package osctrl

import (
	"fmt"
	"strings"

	tk "modernc.org/tk9.0"
)

type unsupportedDriver struct{}

func (d *unsupportedDriver) Name() string {
	return "unsupported"
}

func (d *unsupportedDriver) StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error) {
	return nil, fmt.Errorf("global hotkey is not supported on this OS")
}

func (d *unsupportedDriver) MinimizeMainWindow() {
	tk.WmIconify(tk.App)
	tk.Update()
}

func (d *unsupportedDriver) RestoreMainWindow() {
	restoreMainWindowToForeground()
}

func (d *unsupportedDriver) LaunchBrowser(url string) error {
	target := strings.TrimSpace(url)
	if target == "" {
		target = "about:blank"
	}
	return fmt.Errorf("launch browser is not supported on this OS")
}

func (d *unsupportedDriver) OpenPath(path string) error {
	return fmt.Errorf("open path is not supported on this OS")
}
