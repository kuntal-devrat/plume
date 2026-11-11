package raster

import (
	"math"
)

// BlendMode defines the compositing and blending operators per W3C Compositing
// and Blending Level 1 specification.
type BlendMode uint8

const (
	// Porter-Duff modes
	BlendModeClear BlendMode = iota
	BlendModeSrc
	BlendModeDst
	BlendModeSrcOver
	BlendModeDstOver
	BlendModeSrcIn
	BlendModeDstIn
	BlendModeSrcOut
	BlendModeDstOut
	BlendModeSrcAtop
	BlendModeDstAtop
	BlendModeXor

	// Advanced separable blend modes
	BlendModeMultiply
	BlendModeScreen
	BlendModeOverlay
	BlendModeDarken
	BlendModeLighten
	BlendModeColorDodge
	BlendModeColorBurn
	BlendModeHardLight
	BlendModeSoftLight
	BlendModeDifference
	BlendModeExclusion
)

// Blend composites premultiplied source (sr, sg, sb, sa) over premultiplied
// destination (dr, dg, db, da) using the specified BlendMode.
func Blend(dr, dg, db, da, sr, sg, sb, sa uint8, mode BlendMode) (r, g, b, a uint8) {
	if mode == BlendModeSrcOver {
		return BlendSrcOver(dr, dg, db, da, sr, sg, sb, sa)
	}

	// Normalize to [0, 1] float32 for general compositing
	sR, sG, sB, sA := float32(sr)/255.0, float32(sg)/255.0, float32(sb)/255.0, float32(sa)/255.0
	dR, dG, dB, dA := float32(dr)/255.0, float32(dg)/255.0, float32(db)/255.0, float32(da)/255.0

	var outR, outG, outB, outA float32

	switch mode {
	case BlendModeClear:
		return 0, 0, 0, 0

	case BlendModeSrc:
		return sr, sg, sb, sa

	case BlendModeDst:
		return dr, dg, db, da

	case BlendModeDstOver:
		invDA := 1.0 - dA
		outR = dR + sR*invDA
		outG = dG + sG*invDA
		outB = dB + sB*invDA
		outA = dA + sA*invDA

	case BlendModeSrcIn:
		outR = sR * dA
		outG = sG * dA
		outB = sB * dA
		outA = sA * dA

	case BlendModeDstIn:
		outR = dR * sA
		outG = dG * sA
		outB = dB * sA
		outA = dA * sA

	case BlendModeSrcOut:
		invDA := 1.0 - dA
		outR = sR * invDA
		outG = sG * invDA
		outB = sB * invDA
		outA = sA * invDA

	case BlendModeDstOut:
		invSA := 1.0 - sA
		outR = dR * invSA
		outG = dG * invSA
		outB = dB * invSA
		outA = dA * invSA

	case BlendModeSrcAtop:
		outR = sR*dA + dR*(1.0-sA)
		outG = sG*dA + dG*(1.0-sA)
		outB = sB*dA + dB*(1.0-sA)
		outA = dA

	case BlendModeDstAtop:
		outR = dR*sA + sR*(1.0-dA)
		outG = dG*sA + sG*(1.0-dA)
		outB = dB*sA + sB*(1.0-dA)
		outA = sA

	case BlendModeXor:
		invDA := 1.0 - dA
		invSA := 1.0 - sA
		outR = sR*invDA + dR*invSA
		outG = sG*invDA + dG*invSA
		outB = sB*invDA + dB*invSA
		outA = sA*invDA + dA*invSA

	default:
		// Advanced separable blend modes:
		// Unpremultiply color components before applying non-linear formula
		var ucSR, ucSG, ucSB float32
		if sA > 0 {
			ucSR, ucSG, ucSB = sR/sA, sG/sA, sB/sA
		}
		var ucDR, ucDG, ucDB float32
		if dA > 0 {
			ucDR, ucDG, ucDB = dR/dA, dG/dA, dB/dA
		}

		fR := blendComponent(ucSR, ucDR, mode)
		fG := blendComponent(ucSG, ucDG, mode)
		fB := blendComponent(ucSB, ucDB, mode)

		// Re-premultiply per W3C formula:
		// Cr = (1 - da)*Cs + (1 - sa)*Cd + sa*da*B(Cs, Cd)
		outR = (1.0-dA)*sR + (1.0-sA)*dR + sA*dA*fR
		outG = (1.0-dA)*sG + (1.0-sA)*dG + sA*dA*fG
		outB = (1.0-dA)*sB + (1.0-sA)*dB + sA*dA*fB
		outA = sA + dA*(1.0-sA)
	}

	return toByte(outR), toByte(outG), toByte(outB), toByte(outA)
}

func blendComponent(s, d float32, mode BlendMode) float32 {
	switch mode {
	case BlendModeMultiply:
		return s * d
	case BlendModeScreen:
		return s + d - s*d
	case BlendModeOverlay:
		return hardLight(d, s)
	case BlendModeDarken:
		if s < d {
			return s
		}
		return d
	case BlendModeLighten:
		if s > d {
			return s
		}
		return d
	case BlendModeColorDodge:
		if d == 0 {
			return 0
		}
		if s >= 1.0 {
			return 1.0
		}
		res := d / (1.0 - s)
		if res > 1.0 {
			return 1.0
		}
		return res
	case BlendModeColorBurn:
		if d >= 1.0 {
			return 1.0
		}
		if s <= 0 {
			return 0
		}
		res := 1.0 - (1.0-d)/s
		if res < 0 {
			return 0
		}
		return res
	case BlendModeHardLight:
		return hardLight(s, d)
	case BlendModeSoftLight:
		if s <= 0.5 {
			return d - (1.0-2.0*s)*d*(1.0-d)
		}
		var dVal float32
		if d <= 0.25 {
			dVal = ((16.0*d-12.0)*d + 4.0) * d
		} else {
			dVal = float32(math.Sqrt(float64(d)))
		}
		return d + (2.0*s-1.0)*(dVal-d)
	case BlendModeDifference:
		diff := s - d
		if diff < 0 {
			return -diff
		}
		return diff
	case BlendModeExclusion:
		return s + d - 2.0*s*d
	default:
		return s
	}
}

func hardLight(s, d float32) float32 {
	if s <= 0.5 {
		return 2.0 * s * d
	}
	return 1.0 - 2.0*(1.0-s)*(1.0-d)
}

func toByte(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1.0 {
		return 255
	}
	return uint8(v*255.0 + 0.5)
}
