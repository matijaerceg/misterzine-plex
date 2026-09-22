#include "textflag.h"

// func blendRow(dst, a, b unsafe.Pointer, n4 int, ta, tb uint32)
// dst[i] = (a[i]*ta + b[i]*tb) >> 8 per byte, four pixels (16 bytes) per
// iteration, with NEON. The Go assembler has no NEON mnemonics for 32-bit
// ARM, so the vector instructions are encoded as words (assembled with the
// GNU toolchain; see tools/blend.S).
TEXT ·blendRow(SB),NOSPLIT,$0-24
	MOVW dst+0(FP), R0
	MOVW a+4(FP), R1
	MOVW b+8(FP), R2
	MOVW n4+12(FP), R3
	MOVW ta+16(FP), R4
	MOVW tb+20(FP), R5
	WORD $0xeec44b10 // vdup.8   d4, r4
	WORD $0xeec55b10 // vdup.8   d5, r5
loop:
	WORD $0xf4210a0d // vld1.8   {d0-d1}, [r1]!
	WORD $0xf4222a0d // vld1.8   {d2-d3}, [r2]!
	WORD $0xf3806c04 // vmull.u8 q3, d0, d4
	WORD $0xf3826805 // vmlal.u8 q3, d2, d5
	WORD $0xf3818c04 // vmull.u8 q4, d1, d4
	WORD $0xf3838805 // vmlal.u8 q4, d3, d5
	WORD $0xf2880816 // vshrn.i16 d0, q3, #8
	WORD $0xf2881818 // vshrn.i16 d1, q4, #8
	WORD $0xf4000a0d // vst1.8   {d0-d1}, [r0]!
	SUB.S $1, R3
	BNE loop
	RET
