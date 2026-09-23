package ui

import (
	"time"

	"plexcrt/internal/gfx"
)

// drawScreen paints the top screen and the beta mark over it.
func (a *App) drawScreen(c *gfx.Canvas, now time.Time) bool {
	defer a.drawBetaBrand(c)
	return a.top().Draw(c, now)
}
