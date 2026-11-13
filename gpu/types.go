// Package gpu provides vector path triangulation and hardware-accelerated
// rendering abstractions for the Plume engine.
//
// It enables direct translation of vector shapes ([geom.Path]) into GPU-friendly
// vertex and index buffers (triangles, quads) suitable for Vulkan, Metal,
// OpenGL, and WebGL backends without Cgo.
package gpu

import (
	"github.com/kuntal-devrat/plume"
)

// Vertex represents a 2D textured colored vertex ready for GPU upload.
type Vertex struct {
	X, Y float32 // Position in normalized device coords or screen pixels
	U, V float32 // Texture coordinates
	R, G, B, A uint8 // Vertex color
}

// Mesh holds an indexed triangle list.
type Mesh struct {
	Vertices []Vertex
	Indices  []uint32
}

// Clear resets the mesh buffers for reuse without reallocation.
func (m *Mesh) Clear() {
	m.Vertices = m.Vertices[:0]
	m.Indices = m.Indices[:0]
}

// Texture represents a GPU texture handle.
type Texture interface {
	Width() int
	Height() int
	Release()
}

// DrawCall represents a batched GPU rendering command.
type DrawCall struct {
	IndexOffset uint32
	IndexCount  uint32
	Paint       plume.Paint
	Texture     Texture
}

// Device abstracts driver-specific GPU rendering commands.
type Device interface {
	CreateTexture(width, height int, pixels []byte) (Texture, error)
	Submit(mesh *Mesh, drawCalls []DrawCall) error
}
