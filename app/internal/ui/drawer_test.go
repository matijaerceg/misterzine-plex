package ui

import (
	"plexcrt/internal/gfx"
	"testing"
	"time"
)

func TestDrawerSlideCoversFrameWithoutSeams(t *testing.T) {
	now := time.Now()
	back := gfx.NewCanvas(720, 480)
	back.Fill(0, 0, 720, 480, 0x123456)
	panel := gfx.NewCanvas(DrawerW, 480)
	panel.Fill(0, 0, DrawerW, 480, 0xabcdef)
	d := &Drawer{back: &gfx.Image{W: 720, H: 480, Pix: back.Pix}, page: panel, key: "0", openAt: now}
	for _, elapsed := range []time.Duration{0, 40 * time.Millisecond, 80 * time.Millisecond, animDur} {
		c := gfx.NewCanvas(720, 480)
		c.Fill(0, 0, c.W, c.H, 0xff00ff)
		d.Draw(c, now.Add(elapsed))
		off, _ := d.offset(now.Add(elapsed))
		edge := DrawerW + off
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				want := byte(0x56)
				if x < edge {
					want = 0xef
				}
				if c.Pix[(y*c.W+x)*4] != want {
					t.Fatalf("unpainted or incorrect pixel at %d,%d during %v", x, y, elapsed)
				}
			}
		}
	}
}

func TestDrawerEarlyCloseDoesNotJumpFullyOpen(t *testing.T) {
	d := &Drawer{openAt: time.Now().Add(-20 * time.Millisecond)}
	d.Close(nil)
	start, moving := d.offset(d.closeAt)
	if !moving || start != d.closeFrom || start >= 0 {
		t.Fatal("closing jumped to fully open position")
	}
	halfway, _ := d.offset(d.closeAt.Add(animDur / 2))
	if halfway >= start || halfway <= -DrawerW {
		t.Fatal("closing did not move smoothly from interrupted opening")
	}
	end, moving := d.offset(d.closeAt.Add(animDur))
	if moving || end != -DrawerW {
		t.Fatal("drawer failed to finish closing")
	}
}
