package gfx

import "testing"

func TestBlend(t *testing.T) {
	w, h := 37, 3
	a := &Image{W: w, H: h, Pix: make([]byte, w*h*4)}
	b := &Image{W: w, H: h, Pix: make([]byte, w*h*4)}
	for i := range a.Pix {
		a.Pix[i] = byte(i * 7)
		b.Pix[i] = byte(255 - i*3)
	}
	c := NewCanvas(w, h)
	for _, tt := range []int{0, 1, 64, 128, 200, 255, 256} {
		c.Blend(0, 0, a, b, tt)
		for i := range c.Pix {
			want := byte((uint32(a.Pix[i])*uint32(256-tt) + uint32(b.Pix[i])*uint32(tt)) >> 8)
			if tt >= 256 {
				want = b.Pix[i]
			}
			if c.Pix[i] != want {
				t.Fatalf("t=%d byte %d: got %d want %d", tt, i, c.Pix[i], want)
			}
		}
	}
}
