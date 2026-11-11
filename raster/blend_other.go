//go:build !amd64

package raster

func blendSolidSpan(dst []uint8, coverage []uint8, srcR, srcG, srcB, srcA uint8) {
	blendSolidSpanGeneric(dst, coverage, srcR, srcG, srcB, srcA)
}
