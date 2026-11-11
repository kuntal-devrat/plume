package raster

// blendSolidSpanGeneric is the portable pure-Go fallback for alpha compositing.
func blendSolidSpanGeneric(dst []uint8, coverage []uint8, srcR, srcG, srcB, srcA uint8) {
	n := len(coverage)
	if n*4 > len(dst) {
		n = len(dst) / 4
	}

	sr := uint16(srcR)
	sg := uint16(srcG)
	sb := uint16(srcB)
	sa := uint16(srcA)

	// Fast path for fully opaque solid fill
	if sa == 255 {
		for i := 0; i < n; i++ {
			cov := uint16(coverage[i])
			if cov == 0 {
				continue
			}

			off := i * 4
			if cov == 255 {
				dst[off] = srcR
				dst[off+1] = srcG
				dst[off+2] = srcB
				dst[off+3] = 255
				continue
			}

			// Modulate by coverage
			curSr := (sr*cov + 128) / 255
			curSg := (sg*cov + 128) / 255
			curSb := (sb*cov + 128) / 255
			curSa := (255*cov + 128) / 255

			invA := 255 - curSa
			dst[off] = uint8(curSr + (uint16(dst[off])*invA+128)/255)
			dst[off+1] = uint8(curSg + (uint16(dst[off+1])*invA+128)/255)
			dst[off+2] = uint8(curSb + (uint16(dst[off+2])*invA+128)/255)
			dst[off+3] = uint8(curSa + (uint16(dst[off+3])*invA+128)/255)
		}
		return
	}

	for i := 0; i < n; i++ {
		cov := uint16(coverage[i])
		if cov == 0 {
			continue
		}

		off := i * 4
		curSr := (sr*cov + 128) / 255
		curSg := (sg*cov + 128) / 255
		curSb := (sb*cov + 128) / 255
		curSa := (sa*cov + 128) / 255

		invA := 255 - curSa
		dst[off] = uint8(curSr + (uint16(dst[off])*invA+128)/255)
		dst[off+1] = uint8(curSg + (uint16(dst[off+1])*invA+128)/255)
		dst[off+2] = uint8(curSb + (uint16(dst[off+2])*invA+128)/255)
		dst[off+3] = uint8(curSa + (uint16(dst[off+3])*invA+128)/255)
	}
}
