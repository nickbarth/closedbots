package automation

import (
	"path/filepath"
	"testing"
)

func TestScreenshotCapturerReturnsEitherImageOrError(t *testing.T) {
	c := &ScreenshotCapturer{}
	path := filepath.Join(t.TempDir(), "screen.png")
	_, err := c.CaptureFullScreen(path)
	// In CI/headless this should error. On desktops it may succeed.
	// This test just guarantees the call path executes without panicking.
	if err != nil {
		t.Logf("capture returned expected runtime-dependent error: %v", err)
	}
}
