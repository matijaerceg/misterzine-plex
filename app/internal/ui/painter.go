package ui

import (
	"log"
	"sync"

	"plexcrt/internal/gfx"
)

// painter writes the overlay's pixels from its own goroutine, so the
// play loop never waits on the write into the frame memory (which can
// stall behind the decoder) and the sprites keep moving every field.
// The loop fills a free staging buffer outside the lock and hands it
// over with a few stores; the painter returns it when written. Three
// buffers cover fill, pending and writing. Only the latest panel is
// kept; a pending one that was never written goes straight back.
type painter struct {
	mu   sync.Mutex
	cond *sync.Cond
	osd  OSD
	x, y int
	w, h int
	pend *stage // the panel waiting to be written, or nil
	hid  bool   // a hide waits
	free []*stage
	made int
	jobs []func() // run in order with the writes, on the painter's goroutine
}

type stage struct {
	pix   []byte
	alpha []byte
}

func newPainter() *painter {
	p := &painter{}
	p.cond = sync.NewCond(&p.mu)
	go p.run()
	return p
}

// take returns a free staging buffer, making one of the three if needed.
func (p *painter) take() *stage {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.free); n > 0 {
		s := p.free[n-1]
		p.free = p.free[:n-1]
		return s
	}
	if p.made < 3 {
		p.made++
		return &stage{pix: make([]byte, 720*480*4), alpha: make([]byte, 720*480)}
	}
	return nil // fill, pending and writing all in use: this panel is dropped
}

// show copies the canvas into a free buffer and makes it the pending panel.
func (p *painter) show(osd OSD, x, y int, c *gfx.Canvas, alpha []byte) {
	s := p.take()
	if s == nil {
		return
	}
	n := c.W * c.H
	copy(s.pix, c.Pix[:n*4])
	if alpha != nil {
		copy(s.alpha, alpha[:n])
	} else {
		for i := range s.alpha[:n] {
			s.alpha[i] = 255
		}
	}
	p.mu.Lock()
	if p.pend != nil {
		p.free = append(p.free, p.pend) // never written: superseded
	}
	p.pend = s
	p.osd, p.x, p.y, p.w, p.h = osd, x, y, c.W, c.H
	p.hid = false
	p.mu.Unlock()
	p.cond.Signal()
}

// do queues work for the painter's goroutine, in order with its writes:
// an upload and the header store that shows it must not overtake a
// panel write still in progress, nor the other way round.
func (p *painter) do(f func()) {
	p.mu.Lock()
	p.jobs = append(p.jobs, f)
	p.mu.Unlock()
	p.cond.Signal()
}

// hide queues a hide, cancelling any panel waiting.
func (p *painter) hide(osd OSD) {
	p.mu.Lock()
	if p.pend != nil {
		p.free = append(p.free, p.pend)
		p.pend = nil
	}
	p.osd = osd
	p.hid = true
	p.mu.Unlock()
	p.cond.Signal()
}

func (p *painter) run() {
	defer func() {
		// a bad panel must not take the app down mid-playback
		if r := recover(); r != nil {
			log.Printf("painter: %v", r)
			p.mu.Lock()
			p.pend, p.hid = nil, false
			p.mu.Unlock()
			go p.run()
		}
	}()
	c := &gfx.Canvas{}
	for {
		p.mu.Lock()
		for p.pend == nil && !p.hid && len(p.jobs) == 0 {
			p.cond.Wait()
		}
		if len(p.jobs) > 0 {
			f := p.jobs[0]
			p.jobs = p.jobs[1:]
			p.mu.Unlock()
			f()
			continue
		}
		osd := p.osd
		if p.hid {
			p.hid = false
			p.mu.Unlock()
			osd.Hide()
			continue
		}
		s := p.pend
		p.pend = nil
		x, y, w, h := p.x, p.y, p.w, p.h
		p.mu.Unlock()
		c.W, c.H, c.Pix = w, h, s.pix[:w*h*4]
		osd.Show(x, y, c, s.alpha[:w*h])
		p.mu.Lock()
		p.free = append(p.free, s)
		p.mu.Unlock()
	}
}
