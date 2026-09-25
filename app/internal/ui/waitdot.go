package ui

import (
	"strconv"
	"strings"
	"time"

	"plexcrt/internal/gfx"
)

// The wait dot: while the picture is held up (a playback coming up, or a
// stream that has stopped delivering frames after a track change or a
// restart), the core's dot sprite runs back and forth along a short path
// in the middle of the screen, at an even speed, until the next frame is
// on screen. A seek's wait has none: the strip already says where it goes. The core moves it every field by itself; the app
// only turns it round at the ends, so a late wake shows as a moment's
// rest at the wall, never a stutter.
const (
	WaitDotSpeed = 2   // pixels per field: whole pixels keep the motion even on the tube
	WaitDotPath  = 36  // pixels from one end to the other
	WaitDotY     = 240 // the middle of the frame
	// StallDelay is how long a playing picture may stand still before the
	// dot comes up, so a quick seek never flashes it.
	StallDelay = 500 * time.Millisecond
	// EndSlack is how near the end a stall is the stream finishing, as the
	// launcher counts it (END_SLACK in plexplay.py).
	EndSlack = 5
	// RestartGap is a gap between frames that only a new stream makes: the
	// frame after it ends a seek's hush. SeekHush ends one that never
	// restarted, so a later stall is not taken for the seek's.
	RestartGap = 250 * time.Millisecond
	SeekHush   = 20 * time.Second
)

type waitDot struct {
	v, xmin, xmax, y int
	vx               int // the run last sent: +v or -v
	fields           int // fields stepped so far
}

// newWaitDot lays the path out from a "speed,path" spec (PLEXCRT_WAIT_DOT,
// for tuning on a set); an empty or unreadable part keeps its default.
func newWaitDot(spec string) *waitDot {
	v, path := WaitDotSpeed, WaitDotPath
	parts := strings.Split(spec, ",")
	if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		v = max(1, min(n, 8))
	}
	if len(parts) > 1 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			path = max(2, min(n, SafeW))
		}
	}
	path = max(v, path/v*v) // whole steps from end to end: no short step at a turn
	xmin := SafeX + (SafeW-path)/2
	return &waitDot{v: v, xmin: xmin, xmax: xmin + path, y: WaitDotY, vx: v}
}

// step runs once a field: the dot shows, and turns round at an end.
func (d *waitDot) step(osd OSD) {
	if d.fields > 0 { // the first field's place is from before the start
		if x := osd.DotX(); d.vx > 0 && x >= d.xmax {
			d.vx = -d.v
		} else if d.vx < 0 && x <= d.xmin {
			d.vx = d.v
		}
	}
	// Hidden for the first field, while the core loads the left end (or,
	// if that was already the place last asked for, clamps its dot onto
	// the path). Both words go every field: they are single stores, and a
	// framebuffer mode write that wipes them is repaired at once.
	osd.Dot(d.xmin, d.y, uint32(gfx.Amber), d.fields > 0)
	osd.DotRun(d.vx, d.xmin, d.xmax)
	d.fields++
}

// stop hides the dot and ends its run. The place asked for stays the
// same: a scrub that takes the dot in the same field asks for its own,
// and two new places within one field would cancel out at the core.
func (d *waitDot) stop(osd OSD) {
	osd.Dot(d.xmin, d.y, 0, false)
	osd.DotRun(0, SafeX, SafeX+SafeW)
}

// heldUp reports whether the picture is held up: the start, or no frame
// for StallDelay while playing. Not once the playback is ending, not for
// a seek, not over a scrub (the dot is its cursor) or a list, and not in
// the last seconds, where a stall is the stream finishing.
func (p *Playing) heldUp(now time.Time) bool {
	switch {
	case p.ending:
		return false
	case p.starting:
		return true
	case p.paused || p.frameAt.IsZero() || p.scrub || p.list != nil:
		return false
	case !p.hushAt.IsZero() && now.Sub(p.hushAt) < SeekHush:
		return false
	case p.dur > 0 && p.pos >= p.dur-EndSlack:
		return false
	}
	return now.Sub(p.frameAt) >= StallDelay
}

// waiting starts, steps and stops the wait dot; it runs at the top of
// every Tick, and while the dot runs nothing else in Tick touches it.
func (p *Playing) waiting(now time.Time, osd OSD, field bool) {
	if p.paused {
		p.frameAt = now // a paused picture is not a stall, nor the moment after
	}
	if held := p.heldUp(now); held && p.wait == nil {
		p.wait = newWaitDot(p.waitSpec)
	} else if !held {
		p.stopWait(osd)
	}
	if p.wait != nil && field {
		p.wait.step(osd)
	}
}

// framed notes a frame published by the presenter: a held picture moves
// on, the first one ends the start, and the first of a new stream ends a
// seek's hush.
func (p *Playing) framed(now time.Time, osd OSD) {
	if !p.lastFrame.IsZero() && now.Sub(p.lastFrame) >= RestartGap {
		p.hushAt = time.Time{}
	}
	p.frameAt, p.lastFrame = now, now
	p.starting = false
	p.stopWait(osd)
}

// end is the playback stopping: the dot goes at once and stays gone.
func (p *Playing) end(osd OSD) {
	p.ending = true
	p.stopWait(osd)
}

func (p *Playing) stopWait(osd OSD) {
	if p.wait != nil {
		p.wait.stop(osd)
		p.wait = nil
	}
}
