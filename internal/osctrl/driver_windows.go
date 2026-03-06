//go:build windows

package osctrl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/nickbarth/closedbots/internal/hotkey"
	ghk "golang.design/x/hotkey"
	tk "modernc.org/tk9.0"
)

type windowsDriver struct{}

func (d *windowsDriver) Name() string {
	return "windows-hotkey"
}

func (d *windowsDriver) StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error) {
	if onPress == nil {
		return nil, fmt.Errorf("onPress callback is required")
	}
	parsed, err := hotkey.ParseCombo(combo)
	if err != nil {
		return nil, err
	}
	mods := make([]ghk.Modifier, 0, len(parsed.Modifiers))
	for _, mod := range parsed.Modifiers {
		switch mod {
		case "ctrl":
			mods = append(mods, ghk.ModCtrl)
		case "shift":
			mods = append(mods, ghk.ModShift)
		case "alt":
			mods = append(mods, ghk.ModAlt)
		case "cmd":
			mods = append(mods, ghk.ModWin)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", mod)
		}
	}
	key, err := mapHotkeyKey(parsed.Key)
	if err != nil {
		return nil, err
	}

	hk := ghk.New(mods, key)
	if err := hk.Register(); err != nil {
		return nil, err
	}
	h := &nativeHandle{
		stop: make(chan struct{}),
		done: make(chan struct{}),
		unregister: func() {
			_ = hk.Unregister()
		},
	}
	go func() {
		defer close(h.done)
		for {
			select {
			case <-h.stop:
				return
			case <-hk.Keydown():
				onPress()
			}
		}
	}()
	return h, nil
}

func (d *windowsDriver) MinimizeMainWindow() {
	tk.WmIconify(tk.App)
	tk.Update()
}

func (d *windowsDriver) RestoreMainWindow() {
	restoreMainWindowToForeground()
}

func (d *windowsDriver) LaunchBrowser(url string) error {
	target := strings.TrimSpace(url)
	if target == "" {
		target = "about:blank"
	}
	if p, err := exec.LookPath("chrome"); err == nil {
		return exec.Command(p, target).Start()
	}
	for _, p := range []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	} {
		if _, err := os.Stat(p); err == nil {
			return exec.Command(p, target).Start()
		}
	}
	return d.OpenPath(target)
}

func (d *windowsDriver) OpenPath(path string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
}

type nativeHandle struct {
	stop       chan struct{}
	done       chan struct{}
	unregister func()
	once       sync.Once
}

func (h *nativeHandle) Stop() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		if h.unregister != nil {
			h.unregister()
		}
		close(h.stop)
		<-h.done
	})
}
