//go:build linux

package osctrl

func NewDriver() Driver {
	return &linuxDriver{}
}
