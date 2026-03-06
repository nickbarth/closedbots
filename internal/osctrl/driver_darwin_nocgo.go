//go:build darwin && !cgo

package osctrl

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/nickbarth/closedbots/internal/hotkey"
	tk "modernc.org/tk9.0"
)

type darwinDriver struct{}

func (d *darwinDriver) Name() string {
	return "darwin-nocgo"
}

func (d *darwinDriver) StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error) {
	if _, err := hotkey.ParseCombo(combo); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("global stop hotkey on darwin requires cgo-enabled build")
}

func (d *darwinDriver) MinimizeMainWindow() {
	tk.WmIconify(tk.App)
	tk.Update()
}

func (d *darwinDriver) RestoreMainWindow() {
	restoreMainWindowToForeground()
}

func (d *darwinDriver) LaunchBrowser(url string) error {
	target := strings.TrimSpace(url)
	if target == "" {
		target = "about:blank"
	}
	if err := exec.Command("open", "-a", "Google Chrome", target).Start(); err == nil {
		return nil
	}
	return d.OpenPath(target)
}

func (d *darwinDriver) OpenPath(path string) error {
	return exec.Command("open", path).Start()
}
