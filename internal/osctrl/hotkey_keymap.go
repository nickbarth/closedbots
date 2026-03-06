//go:build windows || (darwin && cgo)

package osctrl

import (
	"fmt"
	"strings"
	"unicode"

	ghk "golang.design/x/hotkey"
)

func mapHotkeyKey(key string) (ghk.Key, error) {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return 0, fmt.Errorf("missing key")
	}
	if len(k) == 1 {
		r := rune(k[0])
		if unicode.IsDigit(r) {
			switch k {
			case "0":
				return ghk.Key0, nil
			case "1":
				return ghk.Key1, nil
			case "2":
				return ghk.Key2, nil
			case "3":
				return ghk.Key3, nil
			case "4":
				return ghk.Key4, nil
			case "5":
				return ghk.Key5, nil
			case "6":
				return ghk.Key6, nil
			case "7":
				return ghk.Key7, nil
			case "8":
				return ghk.Key8, nil
			case "9":
				return ghk.Key9, nil
			}
		}
		if unicode.IsLetter(r) {
			switch k {
			case "a":
				return ghk.KeyA, nil
			case "b":
				return ghk.KeyB, nil
			case "c":
				return ghk.KeyC, nil
			case "d":
				return ghk.KeyD, nil
			case "e":
				return ghk.KeyE, nil
			case "f":
				return ghk.KeyF, nil
			case "g":
				return ghk.KeyG, nil
			case "h":
				return ghk.KeyH, nil
			case "i":
				return ghk.KeyI, nil
			case "j":
				return ghk.KeyJ, nil
			case "k":
				return ghk.KeyK, nil
			case "l":
				return ghk.KeyL, nil
			case "m":
				return ghk.KeyM, nil
			case "n":
				return ghk.KeyN, nil
			case "o":
				return ghk.KeyO, nil
			case "p":
				return ghk.KeyP, nil
			case "q":
				return ghk.KeyQ, nil
			case "r":
				return ghk.KeyR, nil
			case "s":
				return ghk.KeyS, nil
			case "t":
				return ghk.KeyT, nil
			case "u":
				return ghk.KeyU, nil
			case "v":
				return ghk.KeyV, nil
			case "w":
				return ghk.KeyW, nil
			case "x":
				return ghk.KeyX, nil
			case "y":
				return ghk.KeyY, nil
			case "z":
				return ghk.KeyZ, nil
			}
		}
	}

	switch k {
	case "space":
		return ghk.KeySpace, nil
	case "tab":
		return ghk.KeyTab, nil
	case "enter", "return":
		return ghk.KeyReturn, nil
	case "esc", "escape":
		return ghk.KeyEscape, nil
	case "delete":
		return ghk.KeyDelete, nil
	case "left":
		return ghk.KeyLeft, nil
	case "right":
		return ghk.KeyRight, nil
	case "up":
		return ghk.KeyUp, nil
	case "down":
		return ghk.KeyDown, nil
	case "f1":
		return ghk.KeyF1, nil
	case "f2":
		return ghk.KeyF2, nil
	case "f3":
		return ghk.KeyF3, nil
	case "f4":
		return ghk.KeyF4, nil
	case "f5":
		return ghk.KeyF5, nil
	case "f6":
		return ghk.KeyF6, nil
	case "f7":
		return ghk.KeyF7, nil
	case "f8":
		return ghk.KeyF8, nil
	case "f9":
		return ghk.KeyF9, nil
	case "f10":
		return ghk.KeyF10, nil
	case "f11":
		return ghk.KeyF11, nil
	case "f12":
		return ghk.KeyF12, nil
	case "f13":
		return ghk.KeyF13, nil
	case "f14":
		return ghk.KeyF14, nil
	case "f15":
		return ghk.KeyF15, nil
	case "f16":
		return ghk.KeyF16, nil
	case "f17":
		return ghk.KeyF17, nil
	case "f18":
		return ghk.KeyF18, nil
	case "f19":
		return ghk.KeyF19, nil
	case "f20":
		return ghk.KeyF20, nil
	default:
		return 0, fmt.Errorf("unsupported hotkey key %q", key)
	}
}
