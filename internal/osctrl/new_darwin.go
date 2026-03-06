//go:build darwin

package osctrl

func NewDriver() Driver {
	return &darwinDriver{}
}
