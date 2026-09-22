package ui

import (
	"plexcrt/internal/gfx"
)

// The abstract page behind a library: a soft four-corner gradient from
// the item's UltraBlurColors (the server's own extraction from the
// poster), held well under the text's brightness and dithered so the
// gentle ramps do not band at 8 bits. Composed off the render thread.

const (
	BlobMax = 62 // the brightest channel any corner may reach, of 255
)

// bayer4 is the ordered-dither threshold matrix.
var bayer4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// blobPage paints the gradient: corners are top-left, top-right,
// bottom-right, bottom-left in 0xRRGGBB.
func blobPage(w, h int, corners [4]uint32) *gfx.Canvas {
	c := gfx.NewCanvas(w, h)
	var cr [4][3]float32
	maxc := float32(1)
	for i, col := range corners {
		cr[i] = [3]float32{float32(col >> 16 & 0xFF), float32(col >> 8 & 0xFF), float32(col & 0xFF)}
		for _, v := range cr[i] {
			if v > maxc {
				maxc = v
			}
		}
	}
	// scale so the brightest channel lands on BlobMax, and lift the floor
	// slightly above the background so the page reads as a surface
	scale := float32(BlobMax) / maxc
	bgR, bgG, bgB := float32(gfx.Bg>>16&0xFF), float32(gfx.Bg>>8&0xFF), float32(gfx.Bg&0xFF)
	for y := 0; y < h; y++ {
		fy := float32(y) / float32(h-1)
		fy = fy * fy * (3 - 2*fy) // smoothstep: more of the page is corner, less is seam
		row := c.Pix[y*w*4 : (y+1)*w*4]
		for x := 0; x < w; x++ {
			fx := float32(x) / float32(w-1)
			fx = fx * fx * (3 - 2*fx)
			d := (float32(bayer4[y&3][x&3]) + 0.5) / 16 // 0..1
			var out [3]float32
			for k := 0; k < 3; k++ {
				top := cr[0][k]*(1-fx) + cr[1][k]*fx
				bot := cr[3][k]*(1-fx) + cr[2][k]*fx
				out[k] = (top*(1-fy) + bot*fy) * scale
			}
			r := out[0] + bgR*0.5
			g := out[1] + bgG*0.5
			b := out[2] + bgB*0.5
			row[x*4] = byte(clamp(b + d))
			row[x*4+1] = byte(clamp(g + d))
			row[x*4+2] = byte(clamp(r + d))
		}
	}
	return c
}

func clamp(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
