package osctrl

// HotkeyHandle controls a running global hotkey registration.
type HotkeyHandle interface {
	Stop()
}

// Driver encapsulates OS-specific controls used by the app.
type Driver interface {
	Name() string
	StartGlobalStopHotkey(combo string, onPress func()) (HotkeyHandle, error)
	MinimizeMainWindow()
	RestoreMainWindow()
	LaunchBrowser(url string) error
	OpenPath(path string) error
}
