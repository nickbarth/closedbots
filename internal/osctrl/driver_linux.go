//go:build linux

package osctrl

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/hotkey"
	tk "modernc.org/tk9.0"
)

type linuxDriver struct{}

const linuxForegroundActivationTimeout = 1200 * time.Millisecond
const linuxForegroundActivationRetryDelay = 120 * time.Millisecond

type linuxActivationCommand struct {
	name string
	args []string
}

func (d *linuxDriver) Name() string {
	return "linux-x11"
}

func (d *linuxDriver) StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error) {
	return hotkey.Start(combo, onPress)
}

func (d *linuxDriver) MinimizeMainWindow() {
	tk.WmIconify(tk.App)
	tk.Update()
}

func (d *linuxDriver) RestoreMainWindow() {
	restoreMainWindowToForeground()
	go func() {
		_ = runLinuxForegroundActivation(AppWindowTitle())
		time.Sleep(linuxForegroundActivationRetryDelay)
		_ = runLinuxForegroundActivation(AppWindowTitle())
	}()
}

func (d *linuxDriver) LaunchBrowser(url string) error {
	target := strings.TrimSpace(url)
	if target == "" {
		target = "about:blank"
	}
	for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium-browser", "chromium"} {
		if _, err := exec.LookPath(bin); err == nil {
			return exec.Command(bin, target).Start()
		}
	}
	return d.OpenPath(target)
}

func (d *linuxDriver) OpenPath(path string) error {
	return exec.Command("xdg-open", path).Start()
}

func runLinuxForegroundActivation(windowTitle string) error {
	title := strings.TrimSpace(windowTitle)
	if title == "" {
		return fmt.Errorf("window title is empty")
	}

	commands := []linuxActivationCommand{
		{name: "wmctrl", args: []string{"-a", title}},
		{name: "wmctrl", args: []string{"-xa", title}},
		{name: "xdotool", args: []string{"search", "--name", title, "windowactivate", "--sync"}},
	}

	var lastErr error
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd.name); err != nil {
			lastErr = err
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), linuxForegroundActivationTimeout)
		err := exec.CommandContext(ctx, cmd.name, cmd.args...).Run()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no linux activation command configured")
	}
	return lastErr
}
