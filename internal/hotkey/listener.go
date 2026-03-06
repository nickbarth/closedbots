//go:build linux

package hotkey

import (
	"fmt"
	"strings"
	"sync"

	"github.com/robotn/xgbutil"
	"github.com/robotn/xgbutil/keybind"
	"github.com/robotn/xgbutil/xevent"
)

type Listener struct {
	xu   *xgbutil.XUtil
	done chan struct{}
	once sync.Once
}

func Start(comboText string, onPress func()) (*Listener, error) {
	if onPress == nil {
		return nil, fmt.Errorf("onPress callback is required")
	}
	combo, err := ParseCombo(comboText)
	if err != nil {
		return nil, err
	}

	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("x11 connection failed: %w", err)
	}
	keybind.Initialize(xu)

	keyStr := toXKeybind(combo)
	if err := keybind.KeyPressFun(func(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
		onPress()
	}).Connect(xu, xu.RootWin(), keyStr, true); err != nil {
		xu.Conn().Close()
		return nil, fmt.Errorf("hotkey bind failed for %q: %w", comboText, err)
	}

	l := &Listener{
		xu:   xu,
		done: make(chan struct{}),
	}
	go func() {
		defer close(l.done)
		xevent.Main(xu)
	}()
	return l, nil
}

func (l *Listener) Stop() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		xevent.Quit(l.xu)
		l.xu.Conn().Close()
		<-l.done
	})
}

func toXKeybind(combo Combo) string {
	parts := make([]string, 0, len(combo.Modifiers)+1)
	for _, mod := range combo.Modifiers {
		switch strings.ToLower(mod) {
		case "ctrl":
			parts = append(parts, "Control")
		case "shift":
			parts = append(parts, "Shift")
		case "alt":
			parts = append(parts, "Mod1")
		case "cmd":
			parts = append(parts, "Mod4")
		}
	}
	parts = append(parts, combo.Key)
	return strings.Join(parts, "-")
}
