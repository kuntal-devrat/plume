package ebiten

import (
	"testing"

	"github.com/plume-engine/plume/layout"
)

type mockEbitenImage struct {
	writtenBytes int
}

func (m *mockEbitenImage) WritePixels(p []byte) {
	m.writtenBytes = len(p)
}

func TestSurfaceZeroCopy(t *testing.T) {
	s := NewSurface(320, 240)
	if s.Canvas() == nil {
		t.Fatal("Surface Canvas is nil")
	}

	pix := s.Pixels()
	expectedLen := 320 * 240 * 4
	if len(pix) != expectedLen {
		t.Fatalf("expected %d pixels bytes, got %d", expectedLen, len(pix))
	}

	mock := &mockEbitenImage{}
	s.FlushTo(mock)
	if mock.writtenBytes != expectedLen {
		t.Errorf("mock wrote %d bytes, want %d", mock.writtenBytes, expectedLen)
	}

	rgba := s.AsRGBA()
	if rgba.Rect.Dx() != 320 || rgba.Rect.Dy() != 240 {
		t.Errorf("AsRGBA dimensions mismatch: %v", rgba.Rect)
	}
}

func TestUIManagerHitTestAndEvents(t *testing.T) {
	tree := layout.NewTree()
	root := tree.NewNode(layout.Style{
		Display: layout.DisplayFlex,
		Width:   layout.Px(400),
		Height:  layout.Px(300),
	})

	button := tree.NewNode(layout.Style{
		Width:  layout.Px(100),
		Height: layout.Px(40),
	})
	tree.AddChild(root, button)
	tree.Compute(400, 300)

	ui := NewUIManager(tree)

	clicked := false
	ui.OnClick(button, func() {
		clicked = true
	})

	hovered := false
	ui.OnHover(button, func(h bool) {
		hovered = h
	})

	// 1. Pointer move over button (e.g. at (50, 20))
	hit := ui.HandlePointerMove(50, 20)
	if hit != button {
		t.Errorf("HandlePointerMove hit = %d, want button %d", hit, button)
	}
	if !hovered {
		t.Error("expected button to be hovered")
	}

	// 2. Pointer move outside button (e.g. at (200, 200))
	hitOut := ui.HandlePointerMove(200, 200)
	if hitOut != root {
		t.Errorf("HandlePointerMove outside hit = %d, want root %d", hitOut, root)
	}
	if hovered {
		t.Error("expected button hover to be false")
	}

	// 3. Pointer down + up on button triggers Click
	ui.HandlePointerDown(50, 20)
	handled := ui.HandlePointerUp(50, 20)
	if !handled || !clicked {
		t.Error("expected click handler to be invoked")
	}
}
