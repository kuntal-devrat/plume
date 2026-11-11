package gpu

import (
	"testing"

	"github.com/plume-engine/plume"
	"github.com/plume-engine/plume/geom"
)

func TestTessellateRect(t *testing.T) {
	p := geom.Rect(10, 20, 100, 50)
	var mesh Mesh

	TessellatePath(p, 0.25, plume.White, &mesh)

	if len(mesh.Vertices) != 4 {
		t.Fatalf("expected 4 vertices for rect, got %d", len(mesh.Vertices))
	}
	if len(mesh.Indices) != 6 {
		t.Fatalf("expected 6 indices (2 triangles), got %d", len(mesh.Indices))
	}

	dev := &MockDevice{}
	dc := []DrawCall{{IndexOffset: 0, IndexCount: 6}}
	if err := dev.Submit(&mesh, dc); err != nil {
		t.Errorf("device submit failed: %v", err)
	}
}

func TestTessellateCircle(t *testing.T) {
	p := geom.Circle(50, 50, 40)
	var mesh Mesh

	TessellatePath(p, 0.5, plume.Hex("#38bdf8"), &mesh)

	if len(mesh.Vertices) < 8 {
		t.Errorf("expected circle to have at least 8 vertices, got %d", len(mesh.Vertices))
	}
	if len(mesh.Indices) < 18 {
		t.Errorf("expected circle to have indices, got %d", len(mesh.Indices))
	}

	// Submit to mock device to verify all indices are within bounds
	dev := &MockDevice{}
	dc := []DrawCall{{IndexOffset: 0, IndexCount: uint32(len(mesh.Indices))}}
	if err := dev.Submit(&mesh, dc); err != nil {
		t.Errorf("device submission failed: %v", err)
	}
}

func TestMockTexture(t *testing.T) {
	dev := &MockDevice{}
	pixels := make([]byte, 16*16*4)
	tex, err := dev.CreateTexture(16, 16, pixels)
	if err != nil {
		t.Fatalf("CreateTexture failed: %v", err)
	}
	if tex.Width() != 16 || tex.Height() != 16 {
		t.Errorf("unexpected texture dimensions: %dx%d", tex.Width(), tex.Height())
	}
	tex.Release()
}
