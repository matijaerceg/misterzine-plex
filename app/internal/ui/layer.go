package ui

import (
	"math"
	"time"

	"plexcrt/internal/gfx"
)

// Shared elements between the season-picker and episode layouts of a show.

// Handoff is a state transition's immutable source layout and logo position.
// The owning Show retains the source; only interrupted fades need a copy.
type Handoff struct {
	Page *gfx.Canvas
	Logo *gfx.Image
	X, Y int
	ArtY int // backdrop alignment of the source layout
}

// transDur is how long a show layout transition takes.
const (
	transitionFrame = 16683 * time.Microsecond // one 59.94 Hz display tick
	transitionSteps = 14
	transDur        = transitionSteps * transitionFrame
)

// snapshot copies a canvas (the outgoing page must not change under the fade).
func snapshot(c *gfx.Canvas) *gfx.Canvas {
	s := gfx.NewCanvas(c.W, c.H)
	copy(s.Pix, c.Pix)
	return s
}

// logoLayer draws a screen's logo over its composed page every frame, so
// the logo can move and dissolve independently of the page: between two
// screens, or when the hero changes.
type logoLayer struct {
	img     *gfx.Image
	x, y    int
	from    *gfx.Image
	fx, fy  int
	start   time.Time
	dur     time.Duration
	scratch *gfx.Canvas
}

// set makes img at x,y the target; a change dissolves over dur.
func (l *logoLayer) set(img *gfx.Image, x, y int, now time.Time, dur time.Duration) {
	if img == l.img && x == l.x && y == l.y {
		return
	}
	if l.img == nil && img == nil {
		l.x, l.y = x, y
		return
	}
	l.from, l.fx, l.fy = l.cur(now)
	l.img, l.x, l.y = img, x, y
	l.start, l.dur = now, dur
}

// enterFrom begins the slide from where the previous screen had its logo.
func (l *logoLayer) enterFrom(h *Handoff, img *gfx.Image, x, y int, now time.Time) {
	l.from, l.fx, l.fy = h.Logo, h.X, h.Y
	l.img, l.x, l.y = img, x, y
	if l.img == nil {
		l.img, l.x, l.y = h.Logo, x, y // ours has not landed: keep theirs moving
	}
	l.start, l.dur = now, transDur
}

// cur is the image and position as shown now (mid-motion: the target's
// position interpolated; the image is the target's).
func (l *logoLayer) cur(now time.Time) (*gfx.Image, int, int) {
	t := l.t(now)
	if t >= 1 {
		return l.img, l.x, l.y
	}
	return l.img, lerp(l.fx, l.x, t), lerp(l.fy, l.y, t)
}

func (l *logoLayer) t(now time.Time) float64 {
	if l.start.IsZero() || l.dur == 0 {
		return 1
	}
	p := float64(now.Sub(l.start)) / float64(l.dur)
	if p >= 1 {
		return 1
	}
	if p < 0 {
		p = 0
	}
	return 1 - math.Pow(1-p, 3)
}

func lerp(a, b int, t float64) int { return a + int(float64(b-a)*t+0.5) }

// draw paints the layer over page onto the frame; true while moving.
func (l *logoLayer) draw(c, page *gfx.Canvas, now time.Time) bool {
	t := l.t(now)
	if t >= 1 {
		if l.img != nil {
			l.imageOver(c, page, l.x, l.y, l.img, 256, nil, 0, 0, 0)
		}
		l.from = nil
		return false
	}
	x, y := lerp(l.fx, l.x, t), lerp(l.fy, l.y, t)
	a := int(t * 256)
	if l.from == l.img {
		l.imageOver(c, page, x, y, l.img, 256, nil, 0, 0, 0)
		return true
	}
	l.imageOver(c, page, x, y, l.img, a, l.from, x, y, 256-a)
	return true
}

// imageOver composites one or two alpha images over a composed page and
// writes the covered rectangle to the frame: the page's pixels are copied
// to a scratch canvas, the images blended there, and the scratch blitted
// (the frame itself is never read).
func (l *logoLayer) imageOver(c, page *gfx.Canvas, x, y int, img *gfx.Image, t int, img2 *gfx.Image, x2, y2, t2 int) {
	x0, y0, x1, y1 := c.W, c.H, 0, 0
	grow := func(im *gfx.Image, px, py int) {
		if im == nil {
			return
		}
		x0, y0 = min(x0, px), min(y0, py)
		x1, y1 = max(x1, px+im.W), max(y1, py+im.H)
	}
	grow(img, x, y)
	grow(img2, x2, y2)
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, c.W), min(y1, c.H)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	w, h := x1-x0, y1-y0
	if l.scratch == nil || len(l.scratch.Pix) < w*h*4 {
		l.scratch = gfx.NewCanvas(max(w, LogoW), max(h, LogoH))
	}
	strip := &gfx.Canvas{W: w, H: h, Pix: l.scratch.Pix[:w*h*4]}
	for yy := 0; yy < h; yy++ {
		so := ((y0+yy)*page.W + x0) * 4
		copy(strip.Pix[yy*w*4:(yy+1)*w*4], page.Pix[so:so+w*4])
	}
	if img2 != nil && t2 > 0 {
		strip.BlitOverT(x2-x0, y2-y0, img2, t2)
	}
	if img != nil && t > 0 {
		strip.BlitOverT(x-x0, y-y0, img, t)
	}
	c.Blit(x0, y0, &gfx.Image{W: w, H: h, Pix: strip.Pix})
}
