package raster

import (
	"math"
)

// srgbToLinear and linearToSrgb lookup tables for zero-allocation gamma mapping.
var (
	srgbToLinearTable [256]float32
	linearToSrgbTable [1024]uint8
)

func init() {
	// IEC 61966-2-1 standard sRGB transfer curve
	for i := 0; i < 256; i++ {
		c := float64(i) / 255.0
		var lin float64
		if c <= 0.04045 {
			lin = c / 12.92
		} else {
			lin = math.Pow((c+0.055)/1.055, 2.4)
		}
		srgbToLinearTable[i] = float32(lin)
	}

	for i := 0; i < 1024; i++ {
		lin := float64(i) / 1023.0
		var c float64
		if lin <= 0.0031308 {
			c = lin * 12.92
		} else {
			c = 1.055*math.Pow(lin, 1.0/2.4) - 0.055
		}
		if c < 0 {
			c = 0
		} else if c > 1.0 {
			c = 1.0
		}
		linearToSrgbTable[i] = uint8(c*255.0 + 0.5)
	}
}

// SrgbToLinear converts an 8-bit non-linear sRGB color component to linear radiance [0, 1].
func SrgbToLinear(c uint8) float32 {
	return srgbToLinearTable[c]
}

// LinearToSrgb converts a linear radiance value [0, 1] back to an 8-bit sRGB color component.
func LinearToSrgb(lin float32) uint8 {
	if lin <= 0 {
		return 0
	}
	if lin >= 1.0 {
		return 255
	}
	var c float64
	if lin <= 0.0031308 {
		c = float64(lin) * 12.92
	} else {
		c = 1.055*math.Pow(float64(lin), 1.0/2.4) - 0.055
	}
	if c <= 0 {
		return 0
	}
	if c >= 1.0 {
		return 255
	}
	return uint8(c*255.0 + 0.5)
}

// BlendLinearSrcOver performs source-over alpha compositing in linear color space.
// It converts sRGB to linear, composites premultiplied linear radiance, and converts
// back to sRGB. This prevents dark boundary fringe artifacts along anti-aliased edges.
func BlendLinearSrcOver(dstR, dstG, dstB, dstA, srcR, srcG, srcB, srcA uint8) (r, g, b, a uint8) {
	if srcA == 0 {
		return dstR, dstG, dstB, dstA
	}
	if srcA == 255 {
		return srcR, srcG, srcB, 255
	}

	// Destination linear
	dA := float32(dstA) / 255.0
	dR := SrgbToLinear(dstR) * dA
	dG := SrgbToLinear(dstG) * dA
	dB := SrgbToLinear(dstB) * dA

	// Source linear
	sA := float32(srcA) / 255.0
	sR := SrgbToLinear(srcR) * sA
	sG := SrgbToLinear(srcG) * sA
	sB := SrgbToLinear(srcB) * sA

	// Composite in linear space
	invSA := 1.0 - sA
	outA := sA + dA*invSA
	if outA <= 0 {
		return 0, 0, 0, 0
	}

	outR := sR + dR*invSA
	outG := sG + dG*invSA
	outB := sB + dB*invSA

	// Unpremultiply before converting back to sRGB
	resR := LinearToSrgb(outR / outA)
	resG := LinearToSrgb(outG / outA)
	resB := LinearToSrgb(outB / outA)
	resA := uint8(outA*255.0 + 0.5)

	// Re-premultiply in sRGB
	aF := float32(resA) / 255.0
	return uint8(float32(resR)*aF + 0.5), uint8(float32(resG)*aF + 0.5), uint8(float32(resB)*aF + 0.5), resA
}
