package raster

// blender.go implements Porter-Duff source-over alpha compositing with
// premultiplied RGBA color values.
//
// All color channels are expected to be premultiplied by alpha:
//   premultiplied_R = original_R × (A / 255)
//
// The source-over formula for premultiplied colors is:
//   out = src + dst × (1 − srcA/255)
//
// This avoids an extra multiply by srcA on the source side.

// BlendSrcOver composites a single premultiplied RGBA source pixel over a
// premultiplied RGBA destination pixel using the Porter-Duff source-over
// operator.
//
// Returns the resulting premultiplied RGBA components.
func BlendSrcOver(dstR, dstG, dstB, dstA, srcR, srcG, srcB, srcA uint8) (r, g, b, a uint8) {
	// invA = 255 - srcA, the complement of the source alpha.
	invA := uint16(255 - srcA)

	// out_c = src_c + dst_c * (255 - srcA) / 255
	// We add 128 before dividing for rounding to nearest.
	r = srcR + uint8((uint16(dstR)*invA+128)/255)
	g = srcG + uint8((uint16(dstG)*invA+128)/255)
	b = srcB + uint8((uint16(dstB)*invA+128)/255)
	a = srcA + uint8((uint16(dstA)*invA+128)/255)
	return
}

// BlendPixel composites a source color at a given alpha coverage onto a
// destination pixel stored in an RGBA byte buffer.
//
// Parameters:
//   - dst:      RGBA pixel buffer (4 bytes per pixel, premultiplied).
//   - offset:   Byte offset into dst of the target pixel (must be 4-aligned).
//   - srcR–srcA: Premultiplied source color components.
//   - coverage: Alpha coverage from the rasterizer (0 = fully transparent,
//     255 = fully opaque). This modulates the source alpha before
//     compositing.
//
// The effective source pixel becomes (srcR×cov/255, srcG×cov/255,
// srcB×cov/255, srcA×cov/255), which is then composited over the
// destination with source-over.
func BlendPixel(dst []uint8, offset int, srcR, srcG, srcB, srcA uint8, coverage uint8) {
	if coverage == 0 {
		return
	}

	cov := uint16(coverage)

	// Modulate source by coverage.
	sr := uint8((uint16(srcR)*cov + 128) / 255)
	sg := uint8((uint16(srcG)*cov + 128) / 255)
	sb := uint8((uint16(srcB)*cov + 128) / 255)
	sa := uint8((uint16(srcA)*cov + 128) / 255)

	// Read destination.
	dr := dst[offset]
	dg := dst[offset+1]
	db := dst[offset+2]
	da := dst[offset+3]

	// Composite source-over.
	dst[offset], dst[offset+1], dst[offset+2], dst[offset+3] = BlendSrcOver(dr, dg, db, da, sr, sg, sb, sa)
}
