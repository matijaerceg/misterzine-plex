package ui

import (
	"plexcrt/internal/gfx"
	"plexcrt/internal/plex"
	"testing"
	"time"
)

func TestDrawerScrollsToKeepSelectionVisible(t *testing.T) {
	var items []*plex.Item
	for i := 0; i < DrawerRows+11; i++ {
		items = append(items, &plex.Item{Title: "Library " + itoa(i), Type: "section", Key: itoa(i)})
	}
	// opening on a deep entry lands with it in view, not on the first page
	d := &Drawer{items: items, cur: DrawerRows + 5}
	d.scroll()
	if d.cur < d.first || d.cur >= d.first+DrawerRows {
		t.Fatalf("opened with the selection out of view: cur %d, first %d", d.cur, d.first)
	}
	d.cur, d.first = 0, 0
	for i := range items {
		d.cur = i
		d.scroll()
		if d.cur < d.first || d.cur >= d.first+DrawerRows {
			t.Fatalf("selection %d out of window starting %d", d.cur, d.first)
		}
		if d.first > len(items)-DrawerRows {
			t.Fatalf("window past the end: first %d of %d", d.first, len(items))
		}
	}
	for i := len(items) - 1; i >= 0; i-- {
		d.cur = i
		d.scroll()
		if d.cur < d.first {
			t.Fatalf("scrolling up left selection %d above window %d", d.cur, d.first)
		}
	}
	// a short list never scrolls
	s := &Drawer{items: items[:3], cur: 2}
	s.scroll()
	if s.first != 0 {
		t.Fatal("short list scrolled")
	}
}

func TestDrawerSlideCoversFrameWithoutSeams(t *testing.T) {
	now := time.Now()
	back := gfx.NewCanvas(720, 480)
	back.Fill(0, 0, 720, 480, 0x123456)
	panel := gfx.NewCanvas(DrawerW, 480)
	panel.Fill(0, 0, DrawerW, 480, 0xabcdef)
	d := &Drawer{back: &gfx.Image{W: 720, H: 480, Pix: back.Pix}, page: panel, key: "0:0", openAt: now}
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
