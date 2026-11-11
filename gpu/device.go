package gpu

import "fmt"

// MockTexture is a software representation of a GPU texture.
type MockTexture struct {
	width  int
	height int
	data   []byte
}

func (m *MockTexture) Width() int  { return m.width }
func (m *MockTexture) Height() int { return m.height }
func (m *MockTexture) Release()    { m.data = nil }

// MockDevice records submissions and validates buffers in tests.
type MockDevice struct {
	SubmittedDrawCalls int
	TotalVertices      int
	TotalIndices       int
}

// CreateTexture simulates uploading a texture to the GPU.
func (d *MockDevice) CreateTexture(w, h int, pixels []byte) (Texture, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("gpu: invalid texture dimensions %dx%d", w, h)
	}
	return &MockTexture{width: w, height: h, data: pixels}, nil
}

// Submit records the mesh draw submission and verifies index integrity.
func (d *MockDevice) Submit(mesh *Mesh, drawCalls []DrawCall) error {
	if mesh == nil {
		return fmt.Errorf("gpu: nil mesh")
	}

	d.TotalVertices += len(mesh.Vertices)
	d.TotalIndices += len(mesh.Indices)
	d.SubmittedDrawCalls += len(drawCalls)

	// Validate indices are in range
	numVerts := uint32(len(mesh.Vertices))
	for _, dc := range drawCalls {
		end := dc.IndexOffset + dc.IndexCount
		if end > uint32(len(mesh.Indices)) {
			return fmt.Errorf("gpu: draw call index range [%d, %d) out of bounds", dc.IndexOffset, end)
		}
		for i := dc.IndexOffset; i < end; i++ {
			if mesh.Indices[i] >= numVerts {
				return fmt.Errorf("gpu: index %d references invalid vertex %d (total %d)",
					i, mesh.Indices[i], numVerts)
			}
		}
	}

	return nil
}
