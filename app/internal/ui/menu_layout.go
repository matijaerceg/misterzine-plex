package ui

import "plexcrt/internal/gfx"

// Menu content sits farther inside the raster for CRT overscan.
const (
	MenuX      = SafeX + 24
	MenuRight  = SafeX + SafeW - 24
	MenuWidth  = MenuRight - MenuX
	MenuRowH   = 32
	MenuBarGap = 8
)

func menuFocusBar(c *gfx.Canvas, x, y, h int) {
	c.Fill(x-MenuBarGap-BarW, y+2, BarW, h-4, gfx.GreyHi)
}
