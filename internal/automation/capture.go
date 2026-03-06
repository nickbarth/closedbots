package automation

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"github.com/kbinani/screenshot"
)

type Capturer interface {
	CaptureFullScreen(path string) (image.Rectangle, error)
}

type ScreenshotCapturer struct{}

func (c *ScreenshotCapturer) CaptureFullScreen(path string) (image.Rectangle, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return image.Rectangle{}, fmt.Errorf("no active display found")
	}

	bounds := screenshot.GetDisplayBounds(0)
	for i := 1; i < n; i++ {
		bounds = bounds.Union(screenshot.GetDisplayBounds(i))
	}

	canvas := image.NewRGBA(bounds)
	for i := 0; i < n; i++ {
		r := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(r)
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("capture display %d: %w", i, err)
		}
		draw.Draw(canvas, r, img, r.Min, draw.Src)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return image.Rectangle{}, err
	}
	f, err := os.Create(path)
	if err != nil {
		return image.Rectangle{}, err
	}
	defer f.Close()
	if err := png.Encode(f, canvas); err != nil {
		return image.Rectangle{}, err
	}

	return bounds, nil
}
