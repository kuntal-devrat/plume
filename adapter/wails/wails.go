// Package wails provides an integration bridge for Wails v2/v3 desktop apps.
//
// It enables Go backend handlers to render high-DPI vector graphics, charts,
// and UI components using Plume, streaming the results directly to the
// frontend webview via base64 data URLs, shared memory buffers, or custom
// protocol asset servers.
package wails

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/kuntal-devrat/plume/raster"
)

// CanvasBridge formats Plume canvas outputs for webview consumption.
type CanvasBridge struct {
	buf bytes.Buffer
}

// NewCanvasBridge creates a reusable canvas bridge.
func NewCanvasBridge() *CanvasBridge {
	return &CanvasBridge{}
}

// ToPNGDataURL encodes the canvas as a base64 PNG data URL:
// "data:image/png;base64,..." suitable for binding directly to an <img> or Canvas.
func (b *CanvasBridge) ToPNGDataURL(c *raster.Canvas) (string, error) {
	b.buf.Reset()
	b.buf.WriteString("data:image/png;base64,")

	enc := base64.NewEncoder(base64.StdEncoding, &b.buf)
	if err := png.Encode(enc, c.ToRGBA()); err != nil {
		return "", fmt.Errorf("wails: failed to encode PNG: %w", err)
	}
	enc.Close()

	return b.buf.String(), nil
}

// ToRawRGBABase64 encodes the raw pixel buffer into base64 without PNG compression
// for high-framerate WebGL/Canvas2D uploads.
func (b *CanvasBridge) ToRawRGBABase64(c *raster.Canvas) string {
	return base64.StdEncoding.EncodeToString(c.Pixels())
}

// RawPixels returns the direct underlying pixel slice for custom IPC handlers.
func (b *CanvasBridge) RawPixels(c *raster.Canvas) []byte {
	return c.Pixels()
}
