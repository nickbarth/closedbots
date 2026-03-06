//go:build windows

package osctrl

func NewDriver() Driver {
	return &windowsDriver{}
}
