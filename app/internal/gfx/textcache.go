package gfx

import (
	"sync"
)

// TextCache renders text lines off the render thread. A line is an opaque
// strip (text over a known background) so drawing it is one memmove per
// row and never reads the destination, which matters when the destination
// is the write-combined frame ring. A line not yet rendered is queued and
// simply absent for a frame.
type TextCache struct {
	mu    sync.Mutex
	lines map[textKey]*Image
	busy  map[textKey]bool
	work  chan textKey
	wake  chan struct{}
	limit int
}

type textKey struct {
	f       *Font
	s       string
	col, bg Color
}

// NewTextCache starts n render workers; wake is signalled as lines land.
func NewTextCache(n int, wake chan struct{}) *TextCache {
	tc := &TextCache{lines: map[textKey]*Image{}, busy: map[textKey]bool{},
		work: make(chan textKey, 512), wake: wake, limit: 600}
	for i := 0; i < n; i++ {
		go tc.worker()
	}
	return tc
}

func (tc *TextCache) worker() {
	for k := range tc.work {
		w := k.f.Width(k.s) + 2
		if w < 2 {
			w = 2
		}
		c := NewCanvas(w, k.f.Height())
		c.Fill(0, 0, c.W, c.H, k.bg)
		c.Text(1, 0, k.f, k.col, k.s)
		img := &Image{W: c.W, H: c.H, Pix: c.Pix}
		tc.mu.Lock()
		if len(tc.lines) >= tc.limit {
			tc.lines = map[textKey]*Image{} // crude, rare, and the lines come back within a frame
		}
		tc.lines[k] = img
		delete(tc.busy, k)
		tc.mu.Unlock()
		select {
		case tc.wake <- struct{}{}:
		default:
		}
	}
}

// Get returns the rendered line, or nil after queueing it.
func (tc *TextCache) Get(f *Font, s string, col, bg Color) *Image {
	if s == "" {
		return nil
	}
	k := textKey{f, s, col, bg}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if img, ok := tc.lines[k]; ok {
		return img
	}
	if !tc.busy[k] {
		select {
		case tc.work <- k:
			tc.busy[k] = true
		default:
		}
	}
	return nil
}

// Line draws a cached line with its cell top at y (x is the glyph origin).
func (c *Canvas) Line(tc *TextCache, x, y int, f *Font, col, bg Color, s string) {
	if img := tc.Get(f, s, col, bg); img != nil {
		c.Blit(x-1, y, img)
	}
}

// LineClip draws a cached line clipped to rows cy0..cy1.
func (c *Canvas) LineClip(tc *TextCache, x, y int, f *Font, col, bg Color, s string, cy0, cy1 int) {
	if img := tc.Get(f, s, col, bg); img != nil {
		c.BlitClip(x-1, y, img, 0, cy0, c.W, cy1-cy0)
	}
}

// LineClipX draws a cached line clipped to columns cx0..cx1.
func (c *Canvas) LineClipX(tc *TextCache, x, y int, f *Font, col, bg Color, s string, cx0, cx1 int) {
	if img := tc.Get(f, s, col, bg); img != nil {
		c.BlitClip(x-1, y, img, cx0, 0, cx1-cx0, c.H)
	}
}

// LineRight draws a cached line ending at x.
func (c *Canvas) LineRight(tc *TextCache, x, y int, f *Font, col, bg Color, s string) {
	c.Line(tc, x-f.Width(s), y, f, col, bg, s)
}

// LineCenter draws a cached line centred on x.
func (c *Canvas) LineCenter(tc *TextCache, x, y int, f *Font, col, bg Color, s string) {
	c.Line(tc, x-f.Width(s)/2, y, f, col, bg, s)
}

// Over wraps existing pixels as a canvas (a frame-ring slot).
func Over(pix []byte, w, h int) *Canvas { return &Canvas{W: w, H: h, Pix: pix[:w*h*4]} }
