package wails

import (
	"strings"
	"testing"

	"github.com/kuntal-devrat/plume"
	"github.com/kuntal-devrat/plume/raster"
)

func TestWailsBridge(t *testing.T) {
	c := raster.NewCanvas(64, 64)
	c.Clear(plume.Hex("#38bdf8"))

	bridge := NewCanvasBridge()

	dataURL, err := bridge.ToPNGDataURL(c)
	if err != nil {
		t.Fatalf("ToPNGDataURL failed: %v", err)
	}

	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Errorf("data URL missing prefix: %s", dataURL[:30])
	}

	rawB64 := bridge.ToRawRGBABase64(c)
	if len(rawB64) == 0 {
		t.Error("empty raw base64")
	}

	rawPix := bridge.RawPixels(c)
	if len(rawPix) != 64*64*4 {
		t.Errorf("raw pixels len = %d, want %d", len(rawPix), 64*64*4)
	}
}
