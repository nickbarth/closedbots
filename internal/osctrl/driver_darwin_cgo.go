//go:build darwin && cgo

package osctrl

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/nickbarth/closedbots/internal/hotkey"
	ghk "golang.design/x/hotkey"
	hotmain "golang.design/x/hotkey/mainthread"
	tk "modernc.org/tk9.0"
)

type darwinDriver struct{}

func (d *darwinDriver) Name() string {
	return "darwin-hotkey"
}

func (d *darwinDriver) StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error) {
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
			mods = append(mods, ghk.ModOption)
		case "cmd":
			mods = append(mods, ghk.ModCmd)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", mod)
		}
	}
	key, err := mapHotkeyKey(parsed.Key)
	if err != nil {
		return nil, err
	}

	hk := ghk.New(mods, key)
	var regErr error
	hotmain.Call(func() {
		regErr = hk.Register()
	})
	if regErr != nil {
		return nil, regErr
	}

	h := &darwinHotkeyHandle{
		stop: make(chan struct{}),
		done: make(chan struct{}),
		hk:   hk,
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

type darwinHotkeyHandle struct {
	stop chan struct{}
	done chan struct{}
	hk   *ghk.Hotkey
	once sync.Once
}

func (h *darwinHotkeyHandle) Stop() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		if h.hk != nil {
			hotmain.Call(func() {
				_ = h.hk.Unregister()
			})
		}
		close(h.stop)
		<-h.done
	})
}
