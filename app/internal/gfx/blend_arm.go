package gfx

import "unsafe"

// blendRow mixes n4*4 pixels: dst = (a*ta + b*tb) >> 8 per byte (NEON).
//
//go:noescape
func blendRow(dst, a, b unsafe.Pointer, n4 int, ta, tb uint32)
