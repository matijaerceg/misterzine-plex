package ui

import (
	_ "embed"
	"sync"
	"time"

	"plexcrt/internal/gfx"
)

// Original MisterZine startup artwork from misterzine-on-device.
//
//go:embed assets/startup_logo.png
var startupLogoPNG []byte

const startupFade = 750 * time.Millisecond

var splashAsset struct {
	sync.Once
	image *gfx.Image
}

// Decode the premultiplied artwork once before the first frame.
func prepareSplash() {
	splashAsset.Do(func() {
		src, err := gfx.Decode(startupLogoPNG)
		if err != nil {
			panic("invalid embedded startup logo: " + err.Error())
		}
		splashAsset.image = src
	})
}

type startupSplash struct {
	enabled bool
	started bool
	at      time.Time
	logo    *gfx.Image
	page    *gfx.Canvas
}

// Blend in ordinary RAM, then publish without reading the frame ring.
func (a *App) drawScreen(c *gfx.Canvas, now time.Time) bool {
	s := &a.splash
	if !s.enabled {
		return a.top().Draw(c, now)
	}
	if !s.started {
		s.started, s.at = true, now
		prepareSplash()
		s.logo = splashAsset.image
	}
	elapsed := max(time.Duration(0), now.Sub(s.at))
	if elapsed >= startupFade {
		s.enabled, s.logo, s.page = false, nil, nil
		return a.top().Draw(c, now)
	}
	if s.page == nil || s.page.W != c.W || s.page.H != c.H {
		s.page = gfx.NewCanvas(c.W, c.H)
	}
	s.page.Fill(0, 0, c.W, c.H, gfx.Bg)
	a.top().Draw(s.page, now)
	alpha := int(256 * (startupFade - elapsed) / startupFade)
	s.page.BlitOverT((c.W-s.logo.W)/2, (c.H-s.logo.H)/2, s.logo, alpha)
	c.Blit(0, 0, &gfx.Image{W: c.W, H: c.H, Pix: s.page.Pix})
	return true
}
