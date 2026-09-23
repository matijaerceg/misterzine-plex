package ui

import (
	"plexcrt/internal/beta"
	"plexcrt/internal/gfx"
)

// drawBetaBrand is applied to browsing frames only, never player overlays.
func (a *App) drawBetaBrand(c *gfx.Canvas) {
	if !beta.IsBeta() {
		return
	}
	x, y := SafeX+SafeW-2-betaBadgeW, SafeY
	switch a.top().(type) {
	case *Home, *Login, *starting:
		x, y = SafeX+16+a.Mark.W+12, SafeY
	case *Drawer:
		return // drawn on the moving drawer panel
	}
	a.betaBadge(c, x, y)
}

// The badge is drawn at 72 x 28 with the 18 px bold face, then shrunk to
// 41 x 16 so it reads as a mark rather than a label.
const betaBadgeW, betaBadgeH = 41, 16

func (a *App) betaBadge(c *gfx.Canvas, x, y int) {
	if a.betaMark == nil {
		const w, h = 72, 28
		f := gfx.Load("bold18")
		badge := gfx.NewCanvas(w, h)
		badge.Fill(0, 0, w, h, gfx.Amber)
		badge.TextCenter(w/2, (h-f.Height())/2+1, f, gfx.Bg, "BETA")
		full := &gfx.Image{W: w, H: h, Pix: badge.Pix}
		a.betaMark = full.Shrink(betaBadgeW, betaBadgeH)
	}
	c.Blit(x, y, a.betaMark)
}
