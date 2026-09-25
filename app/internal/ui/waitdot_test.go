package ui

import (
	"testing"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
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

// waitDotAt reports whether the wait dot is what the core shows.
func (c *coreOSD) waitDotAt() bool {
	return c.on && c.y == WaitDotY && c.rgb == uint32(gfx.Amber)
}

// runDot steps a wait dot for n fields, the app waking in every field
// but the skipped ones, and returns where the dot showed in each field
// (-1 when hidden).
func runDot(d *waitDot, c *coreOSD, n int, skip func(field int) bool) []int {
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

func TestWaitDotRunsEvenlyBetweenTheEnds(t *testing.T) {
	d := newWaitDot("")
	if d.xmin != 342 || d.xmax != 378 || d.v != 2 || d.y != 240 {
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

func TestWaitDotRestsAtTheWallWhenTheAppIsLate(t *testing.T) {
	d := newWaitDot("")
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

func TestWaitDotSpec(t *testing.T) {
	for _, tc := range []struct {
		spec          string
		v, xmin, xmax int
	}{
		{"", 2, 342, 378},
		{"junk", 2, 342, 378},
		{"4", 4, 342, 378},
		{"3,80", 3, 321, 399}, // 80 is not whole steps of 3: 78
		{" 1 , 120 ", 1, 300, 420},
		{"20,5000", 8, SafeX, SafeX + SafeW}, // clamped to 8 px and the safe width
		{"0,0", 1, 359, 361},
	} {
		d := newWaitDot(tc.spec)
		if d.v != tc.v || d.xmin != tc.xmin || d.xmax != tc.xmax {
			t.Errorf("%q: %d px/field on %d..%d, want %d on %d..%d", tc.spec, d.v, d.xmin, d.xmax, tc.v, tc.xmin, tc.xmax)
		}
		if (d.xmax-d.xmin)%d.v != 0 || (d.xmin+d.xmax)/2 < 358 || (d.xmin+d.xmax)/2 > 362 {
			t.Errorf("%q: path %d..%d is not whole steps of %d about the middle", tc.spec, d.xmin, d.xmax, d.v)
		}
	}
}

// waitPlaying is a playback controller as PlayQueue makes one, drawing
// into a fake core.
func waitPlaying(t *testing.T) *Playing {
	p := playingCrop(cropApp(t), 1.78)
	p.visible = false
	p.dur, p.pos = 1800, 600
	p.canvas, p.alpha, p.painter = gfx.NewCanvas(720, 480), make([]byte, 720*480), newPainter()
	p.starting = true
	return p
}

// tickFields runs Tick for n fields from t0, a field apart, and returns the
// time after them.
func tickFields(p *Playing, c *coreOSD, t0 time.Time, n int, each func(f int)) time.Time {
	for f := 0; f < n; f++ {
		c.vsync()
		p.Tick(t0, c, true)
		if each != nil {
			each(f)
		}
		t0 = t0.Add(16683 * time.Microsecond)
	}
	return t0
}

func TestStartStripKeepsTheDotUntilThePicture(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := tickFields(p, c, time.Now(), 90, func(f int) {
		p.peekAt, p.peekFor = time.Now(), OsdLinger // as playLoop does before the picture
		if f > 0 && !c.waitDotAt() {
			t.Fatalf("field %d: the start strip took the dot down (on %v, y %d)", f, c.on, c.y)
		}
		if c.barOn {
			t.Fatalf("field %d: the bar sprite showed over the start", f)
		}
	})
	p.framed(now, c)
	if c.on || c.vx != 0 || p.wait != nil || p.starting {
		t.Fatalf("the dot outlived the picture: on %v, run %d", c.on, c.vx)
	}
	// the strip lingers over the picture with no dot, and the run stays stopped
	c.vsync()
	p.Tick(now, c, true)
	if c.on || c.vx != 0 {
		t.Fatalf("the strip brought the dot back: on %v, run %d", c.on, c.vx)
	}
}

func TestWaitDotComesUpWhenThePictureStalls(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	t0 := time.Now()
	p.framed(t0, c) // the picture is up
	// frames keep coming: no dot
	now := tickFields(p, c, t0, 60, func(f int) {
		if f%2 == 0 {
			p.framed(t0.Add(time.Duration(f)*16683*time.Microsecond), c)
		}
		if c.on {
			t.Fatalf("field %d: a dot over a moving picture", f)
		}
	})
	// the frames stop (the launcher restarting after a network hiccup):
	// nothing for StallDelay, then the dot
	stall := now
	now = tickFields(p, c, now, 60, nil)
	if !c.waitDotAt() {
		t.Fatal("no dot a second into a stall")
	}
	if p.wait == nil || stall.Add(StallDelay).After(now) {
		t.Fatal("the stall was not timed from the last frame")
	}
	// the new stream's first frame takes it down at once
	p.framed(now, c)
	if c.on || c.vx != 0 {
		t.Fatalf("the dot outlived the stall: on %v, run %d", c.on, c.vx)
	}
}

// playOn publishes a frame every other field for n fields and returns
// the time after them.
func playOn(p *Playing, c *coreOSD, now time.Time, n int) time.Time {
	return tickFields(p, c, now, n, func(f int) {
		if f%2 == 0 {
			p.framed(now.Add(time.Duration(f)*16683*time.Microsecond), c)
		}
	})
}

func TestNoWaitDotForASeek(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	now = playOn(p, c, now, 30)
	p.seekBy(10) // the old stream runs on for a frame or two, then stops
	now = playOn(p, c, now, 4)
	now = tickFields(p, c, now, 120, func(f int) {
		if c.on {
			t.Fatalf("a dot %d fields into a seek's wait", f)
		}
	})
	// the new stream: its first frame ends the seek's hush, so a stall
	// later on (a network hiccup) shows the dot again
	now = playOn(p, c, now, 30)
	tickFields(p, c, now, 60, nil)
	if !c.waitDotAt() {
		t.Fatal("no dot for a stall after the seek was over")
	}
}

func TestWaitDotForATrackChange(t *testing.T) {
	p := waitPlaying(t)
	p.item.Audio = []plex.Stream{{ID: "1", Title: "English", Selected: true}, {ID: "2", Title: "Commentary"}}
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	now = playOn(p, c, now, 30)
	p.openList("Audio")
	p.list.cur = 1
	p.choose(p.list) // the stream starts over with the new track
	if p.list != nil {
		t.Fatal("the list stayed open")
	}
	tickFields(p, c, now, 60, nil)
	if !c.waitDotAt() {
		t.Fatal("no dot while the new track comes up")
	}
}

func TestWaitDotStaysDownForAQuickSeek(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	// a 400 ms gap: under StallDelay
	now = tickFields(p, c, now, 24, func(int) {
		if c.on {
			t.Fatal("a dot in a gap shorter than StallDelay")
		}
	})
	p.framed(now, c)
	tickFields(p, c, now, 10, func(int) {
		if c.on {
			t.Fatal("a dot after the frames came back")
		}
	})
}

func TestWaitDotNotWhenPausedEndingOrStopping(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(p *Playing)
	}{
		{"paused", func(p *Playing) { p.paused = true; p.pausedAt = time.Now() }},
		{"at the end", func(p *Playing) { p.pos = p.dur - 2 }},
		{"stopping", func(p *Playing) { p.ending = true }},
	} {
		p := waitPlaying(t)
		c := &coreOSD{}
		now := time.Now()
		p.framed(now, c)
		tc.set(p)
		tickFields(p, c, now, 90, func(f int) {
			if c.on {
				t.Fatalf("%s: a dot at field %d", tc.name, f)
			}
		})
	}
}

func TestResumeAfterPauseGetsTheFullDelay(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	p.paused, p.pausedAt = true, now
	now = tickFields(p, c, now, 120, nil) // two seconds paused: no frames, no dot
	p.paused = false                      // resumed; the first frame is a moment away
	tickFields(p, c, now, 20, func(f int) {
		if c.on {
			t.Fatalf("a dot %d fields after resuming", f)
		}
	})
}

func TestScrubTakesTheDotFromAStall(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	now = tickFields(p, c, now, 60, nil)
	if !c.waitDotAt() {
		t.Fatal("no dot in the stall")
	}
	// a press of right starts a scrub on the strip: the dot becomes its cursor
	p.Key(input.Event{Key: input.Right}, now)
	p.Tick(now, c, false)
	if p.wait != nil || c.waitDotAt() {
		t.Fatal("the wait dot stayed over a scrub")
	}
	c.vsync()
	want := SafeX + int(float64(SafeW)*p.scrubTo/p.dur+0.5)
	if c.dx != want {
		t.Fatalf("the scrub cursor starts at %d, not at %d: the core never took its place", c.dx, want)
	}
}
