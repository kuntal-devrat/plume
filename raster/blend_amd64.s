#include "textflag.h"

// func cpuid(eaxIn, ecxIn uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL eaxIn+0(FP), AX
	MOVL ecxIn+4(FP), CX
	CPUID
	MOVL AX, eax+8(FP)
	MOVL BX, ebx+12(FP)
	MOVL CX, ecx+16(FP)
	MOVL DX, edx+20(FP)
	RET

// func blendSolidSpanAVX2(dst []uint8, coverage []uint8, srcR, srcG, srcB, srcA uint8)
TEXT ·blendSolidSpanAVX2(SB), NOSPLIT, $0-56
	MOVQ dst_base+0(FP), DI       // DI = dst
	MOVQ dst_len+8(FP), DX        // DX = dst_len
	MOVQ cov_base+24(FP), SI      // SI = coverage
	MOVQ cov_len+32(FP), CX       // CX = cov_len

	// Ensure we don't read beyond dst
	SHRQ $2, DX                   // DX = max pixels in dst
	CMPQ CX, DX
	JLE  len_ok
	MOVQ DX, CX
len_ok:
	CMPQ CX, $0
	JLE  done

	// Load source colors into registers
	MOVBLZX srcR+48(FP), R8
	MOVBLZX srcG+49(FP), R9
	MOVBLZX srcB+50(FP), R10
	MOVBLZX srcA+51(FP), R11

	XORQ AX, AX                   // AX = pixel index i

loop:
	CMPQ AX, CX
	JGE  done

	MOVBLZX (SI)(AX*1), R12       // R12 = cov = coverage[i]
	TESTQ R12, R12
	JZ   next_pix                 // skip if coverage is 0

	LEAQ (DI)(AX*4), BX           // BX = &dst[i*4]

	CMPQ R12, $255
	JNE  partial
	CMPQ R11, $255
	JNE  partial

	// Fast path: fully opaque pixel & full coverage
	MOVB R8, 0(BX)
	MOVB R9, 1(BX)
	MOVB R10, 2(BX)
	MOVB $255, 3(BX)
	JMP  next_pix

partial:
	// curSr = (srcR * cov + 128) / 255
	MOVQ R8, R13
	IMULQ R12, R13
	ADDQ $128, R13
	IMULQ $0x8081, R13           // fast divide by 255 via magic multiply
	SHRQ $23, R13                 // R13 = curSr

	// curSg
	MOVQ R9, R14
	IMULQ R12, R14
	ADDQ $128, R14
	IMULQ $0x8081, R14
	SHRQ $23, R14                 // R14 = curSg

	// curSb
	MOVQ R10, R15
	IMULQ R12, R15
	ADDQ $128, R15
	IMULQ $0x8081, R15
	SHRQ $23, R15                 // R15 = curSb

	// curSa
	MOVQ R11, DX
	IMULQ R12, DX
	ADDQ $128, DX
	IMULQ $0x8081, DX
	SHRQ $23, DX                  // DX = curSa

	// invA = 255 - curSa
	MOVQ $255, R12
	SUBQ DX, R12                  // R12 = invA

	// Composite R: dst[0] = curSr + (dst[0] * invA + 128) / 255
	MOVBLZX 0(BX), AX             // AX = dst[0]
	IMULQ R12, AX
	ADDQ $128, AX
	IMULQ $0x8081, AX
	SHRQ $23, AX
	ADDQ R13, AX
	MOVB AX, 0(BX)

	// Composite G: dst[1]
	MOVBLZX 1(BX), AX
	IMULQ R12, AX
	ADDQ $128, AX
	IMULQ $0x8081, AX
	SHRQ $23, AX
	ADDQ R14, AX
	MOVB AX, 1(BX)

	// Composite B: dst[2]
	MOVBLZX 2(BX), AX
	IMULQ R12, AX
	ADDQ $128, AX
	IMULQ $0x8081, AX
	SHRQ $23, AX
	ADDQ R15, AX
	MOVB AX, 2(BX)

	// Composite A: dst[3]
	MOVBLZX 3(BX), AX
	IMULQ R12, AX
	ADDQ $128, AX
	IMULQ $0x8081, AX
	SHRQ $23, AX
	ADDQ DX, AX
	MOVB AX, 3(BX)

	// Restore AX to loop index
	SUBQ DI, BX
	SHRQ $2, BX
	MOVQ BX, AX

next_pix:
	INCQ AX
	JMP  loop

done:
	RET
