package ui

import (
	"testing"
	"time"

	"plexcrt/internal/gfx"
)

// coreOSD plays the core's part for the dot sprite (rtl/ddr_scanout.v):
// at each vsync it takes the asked x when the load bit has toggled, else
// adds the run's vx, clamped to the run's limits. Dot toggles the load
// bit only for a new x, as ring.Overlay does.
type coreOSD struct {
	x, y           int
	rgb            uint32
	on             bool
	vx, xmin, xmax int
	load, loadQ    bool
	asked          int
	askedSet       bool
	dx             int // where the core has the dot
	barOn          bool
}

func (c *coreOSD) Show(int, int, *gfx.Canvas, []byte) {}
func (c *coreOSD) Hide()                              {}
func (c *coreOSD) Upload(int, *gfx.Canvas, byte)      {}
func (c *coreOSD) ShowAt(int, int, int, int, int)     {}
func (c *coreOSD) Dot(x, y int, rgb uint32, on bool) {
	c.x, c.y, c.rgb, c.on = x, y, rgb, on
	if x != c.asked || !c.askedSet {
		c.asked, c.askedSet = x, true
		c.load = !c.load
	}
}
func (c *coreOSD) DotRun(vx, xmin, xmax int)             { c.vx, c.xmin, c.xmax = vx, xmin, xmax }
func (c *coreOSD) DotX() int                             { return c.dx }
func (c *coreOSD) Bar(_, _, _, _ int, _ uint32, on bool) { c.barOn = on }
func (c *coreOSD) vsync() {
	if c.load != c.loadQ {
		c.loadQ = c.load
		c.dx = c.x
	} else if c.vx != 0 {
		c.dx = max(c.xmin, min(c.dx+c.vx, c.xmax))
	}
}

// runDot steps a start dot for n fields, the app waking in every field
// but the skipped ones, and returns where the dot showed in each field
// (-1 when hidden).
func runDot(d *startDot, c *coreOSD, n int, skip func(field int) bool) []int {
	var seen []int
	for f := 0; f < n; f++ {
		c.vsync()
		if skip == nil || !skip(f) {
			d.step(c)
		}
		if c.on {
			seen = append(seen, c.dx)
		} else {
			seen = append(seen, -1)
		}
	}
	return seen
}

func TestStartDotRunsEvenlyBetweenTheEnds(t *testing.T) {
	d := newStartDot("")
	if d.xmin != 320 || d.xmax != 400 || d.v != 2 || d.y != 240 {
		t.Fatalf("default path %d..%d at %d px/field, y %d", d.xmin, d.xmax, d.v, d.y)
	}
	c := &coreOSD{dx: 600, asked: 999, askedSet: true} // left elsewhere by a scrub
	seen := runDot(d, c, 200, nil)
	if seen[0] != -1 {
		t.Fatalf("the dot showed before the core had it on the path: %d", seen[0])
	}
	if seen[1] != d.xmin {
		t.Fatalf("the dot started at %d, not the left end %d", seen[1], d.xmin)
	}
	ends := map[int]int{}
	for f := 2; f < len(seen); f++ {
		x, was := seen[f], seen[f-1]
		if x < d.xmin || x > d.xmax {
			t.Fatalf("field %d: the dot left the path: %d", f, x)
		}
		if step := x - was; step != d.v && step != -d.v {
			t.Fatalf("field %d: a step of %d, not %d either way (%v)", f, step, d.v, seen[max(0, f-5):f+1])
		}
		if x == d.xmin || x == d.xmax {
			ends[x]++
		}
	}
	if ends[d.xmin] < 2 || ends[d.xmax] < 2 {
		t.Fatalf("the dot did not keep turning at both ends: %v", ends)
	}
}

func TestStartDotRestsAtTheWallWhenTheAppIsLate(t *testing.T) {
	d := newStartDot("")
	c := &coreOSD{}
	// the app misses four fields just as the dot reaches the right end
	arrive := 1 + (d.xmax-d.xmin)/d.v
	seen := runDot(d, c, 120, func(f int) bool { return f > arrive-1 && f <= arrive+3 })
	for f := 2; f < len(seen); f++ {
		if x := seen[f]; x < d.xmin || x > d.xmax {
			t.Fatalf("field %d: the dot left the path: %d", f, x)
		}
		if step := seen[f] - seen[f-1]; step != 0 && step != d.v && step != -d.v {
			t.Fatalf("field %d: a jump of %d", f, step)
		}
	}
	if seen[arrive] != d.xmax || seen[arrive+3] != d.xmax {
		t.Fatalf("the dot did not wait at the wall: %v", seen[arrive-2:arrive+6])
	}
	if seen[arrive+6] >= d.xmax {
		t.Fatalf("the dot did not come back once the app woke: %v", seen[arrive-2:arrive+8])
	}
}

func TestStartDotSpec(t *testing.T) {
	for _, tc := range []struct {
		spec          string
		v, xmin, xmax int
	}{
		{"", 2, 320, 400},
		{"junk", 2, 320, 400},
		{"3", 3, 321, 399}, // 80 is not whole steps of 3: 78
		{"3,80", 3, 321, 399},
		{" 1 , 120 ", 1, 300, 420},
		{"20,5000", 8, SafeX, SafeX + SafeW}, // clamped to 8 px and the safe width
		{"0,0", 1, 359, 361},
	} {
		d := newStartDot(tc.spec)
		if d.v != tc.v || d.xmin != tc.xmin || d.xmax != tc.xmax {
			t.Errorf("%q: %d px/field on %d..%d, want %d on %d..%d", tc.spec, d.v, d.xmin, d.xmax, tc.v, tc.xmin, tc.xmax)
		}
		if (d.xmax-d.xmin)%d.v != 0 || (d.xmin+d.xmax)/2 < 358 || (d.xmin+d.xmax)/2 > 362 {
			t.Errorf("%q: path %d..%d is not whole steps of %d about the middle", tc.spec, d.xmin, d.xmax, d.v)
		}
	}
}

func TestStartStripKeepsTheDotUntilThePicture(t *testing.T) {
	a := cropApp(t)
	p := playingCrop(a, 1.78)
	p.visible = false
	p.dur, p.pos = 1800, 600
	p.canvas, p.alpha, p.painter = gfx.NewCanvas(720, 480), make([]byte, 720*480), newPainter()
	p.start = newStartDot("")
	c := &coreOSD{}
	for f := 0; f < 90; f++ {
		c.vsync()
		p.peekAt, p.peekFor = time.Now(), OsdLinger // as playLoop does before the picture
		p.Tick(time.Now(), c, true)
		if f > 0 && (!c.on || c.y != StartDotY || c.rgb != uint32(gfx.Amber)) {
			t.Fatalf("field %d: the start strip took the dot down (on %v, y %d)", f, c.on, c.y)
		}
		if c.barOn {
			t.Fatalf("field %d: the bar sprite showed over the start", f)
		}
	}
	p.pictureUp(c)
	if c.on || c.vx != 0 || p.start != nil {
		t.Fatalf("the dot outlived the picture: on %v, run %d", c.on, c.vx)
	}
	// the strip lingers over the picture with no dot, and the run stays stopped
	c.vsync()
	p.Tick(time.Now(), c, true)
	if c.on || c.vx != 0 {
		t.Fatalf("the strip brought the dot back: on %v, run %d", c.on, c.vx)
	}
}
