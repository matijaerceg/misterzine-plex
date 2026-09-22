//go:build !arm

package gfx

import "unsafe"

// blendRow mixes n4*4 pixels: dst = (a*ta + b*tb) >> 8 per byte.
func blendRow(dst, a, b unsafe.Pointer, n4 int, ta, tb uint32) {
	d := unsafe.Slice((*byte)(dst), n4*16)
	pa := unsafe.Slice((*byte)(a), n4*16)
	pb := unsafe.Slice((*byte)(b), n4*16)
	for i := range d {
		d[i] = byte((uint32(pa[i])*ta + uint32(pb[i])*tb) >> 8)
	}
}
