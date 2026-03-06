package automation

import "strings"

var robotgoKeyAliases = map[string]string{
	"control":  "ctrl",
	"ctl":      "ctrl",
	"return":   "enter",
	"esc":      "escape",
	"del":      "delete",
	"bksp":     "backspace",
	"spacebar": "space",
	"pgup":     "pageup",
	"pgdn":     "pagedown",
	"option":   "alt",
	"command":  "cmd",
	"windows":  "cmd",
	"super":    "cmd",
	"super_l":  "lcmd",
	"super_r":  "rcmd",
	"meta":     "cmd",
}

func normalizeKeyName(key string) string {
	k := strings.TrimSpace(strings.ToLower(key))
	if k == "" {
		return ""
	}
	if alias, ok := robotgoKeyAliases[k]; ok {
		return alias
	}
	return k
}

func normalizeKeySequence(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		n := normalizeKeyName(key)
		if n == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}
