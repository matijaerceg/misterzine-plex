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
	case *Home, *Login:
		x, y = SafeX+16+a.Mark.W+12, SafeY
	case *Drawer:
		return // drawn on the moving drawer panel
	}
	a.betaBadge(c, x, y)
}

// The badge is drawn at 62 x 24 with the 16 px medium face, then shrunk to
// 60% so it reads as a mark rather than a label.
const betaBadgeW, betaBadgeH = 37, 14

func (a *App) betaBadge(c *gfx.Canvas, x, y int) {
	if a.betaMark == nil {
		badge := gfx.NewCanvas(62, 24)
		badge.Fill(0, 0, 62, 24, gfx.Amber)
		badge.Text(8, 3, a.F.SmallBold, gfx.Bg, "BETA")
		full := &gfx.Image{W: 62, H: 24, Pix: badge.Pix}
		a.betaMark = full.Shrink(betaBadgeW, betaBadgeH)
	}
	c.Blit(x, y, a.betaMark)
}
