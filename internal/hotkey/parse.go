package hotkey

import (
	"fmt"
	"strings"
)

type Combo struct {
	Key       string
	Modifiers []string
}

func ParseCombo(raw string) (Combo, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return Combo{}, fmt.Errorf("hotkey is empty")
	}
	parts := strings.Split(clean, "+")
	if len(parts) < 2 {
		return Combo{}, fmt.Errorf("hotkey must include at least one modifier and one key")
	}

	modSeen := map[string]bool{}
	mods := make([]string, 0, len(parts)-1)
	for i := 0; i < len(parts)-1; i++ {
		mod, ok := normalizeModifier(parts[i])
		if !ok {
			return Combo{}, fmt.Errorf("unsupported modifier %q", parts[i])
		}
		if !modSeen[mod] {
			modSeen[mod] = true
			mods = append(mods, mod)
		}
	}
	key := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if key == "" {
		return Combo{}, fmt.Errorf("missing key")
	}
	return Combo{Key: key, Modifiers: mods}, nil
}

func normalizeModifier(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ctrl", "control":
		return "ctrl", true
	case "shift":
		return "shift", true
	case "alt":
		return "alt", true
	case "cmd", "command", "meta", "super":
		return "cmd", true
	default:
		return "", false
	}
}
