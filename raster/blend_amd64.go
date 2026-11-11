//go:build amd64

package raster

func cpuid(eaxIn, ecxIn uint32) (eax, ebx, ecx, edx uint32)

var useAVX2 = checkAVX2()

func checkAVX2() bool {
	maxID, _, _, _ := cpuid(0, 0)
	if maxID < 7 {
		return false
	}
	_, ebx, _, _ := cpuid(7, 0)
	return (ebx & (1 << 5)) != 0 // AVX2 is bit 5 of EBX on leaf 7
}

func blendSolidSpan(dst []uint8, coverage []uint8, srcR, srcG, srcB, srcA uint8) {
	if useAVX2 && len(coverage) >= 8 {
		blendSolidSpanAVX2(dst, coverage, srcR, srcG, srcB, srcA)
		return
	}
	blendSolidSpanGeneric(dst, coverage, srcR, srcG, srcB, srcA)
}

//go:noescape
func blendSolidSpanAVX2(dst []uint8, coverage []uint8, srcR, srcG, srcB, srcA uint8)
