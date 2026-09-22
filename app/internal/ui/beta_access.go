package ui

import (
	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"time"
)

// BetaAccess explains the offline key workflow without blocking browsing.
type BetaAccess struct{ app *App }

func (b *BetaAccess) Key(ev input.Event, now time.Time) {
	if ev.Key == input.Enter && !ev.Release && !ev.Repeat {
		b.app.Pop()
	}
}

func (b *BetaAccess) Draw(c *gfx.Canvas, now time.Time) bool {
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	b.app.text(c, SafeX, SafeY+36, b.app.F.Title, gfx.White, "Patreon beta access")
	lines := []string{
		"MisterZine Plex Core",
		"Playback in this beta needs a patron key.",
		"Download the key ZIP from the Patreon beta post.",
		"Extract it onto the root of your MiSTer SD card.",
		"Keep the folders, then return here and press Play.",
		"You can keep browsing without a key.",
	}
	for i, line := range lines {
		b.app.text(c, SafeX, SafeY+90+i*34, b.app.F.Body, gfx.White, line)
	}
	b.app.text(c, SafeX, SafeY+320, b.app.F.Body, gfx.Amber, "OK / Back: return to browsing")
	return false
}
