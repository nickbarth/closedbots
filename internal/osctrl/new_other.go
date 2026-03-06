//go:build !linux && !windows && !darwin

package osctrl

func NewDriver() Driver {
	return &unsupportedDriver{}
}
