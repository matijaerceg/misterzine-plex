package ui

import (
	"strconv"
	"strings"

	"plexcrt/internal/gfx"
)

// The start dot: while a playback comes up, the core's dot sprite runs
// back and forth along a short path in the middle of the black, at an even
// speed, until the presenter's first frame is on screen. The core moves it
// every field by itself; the app only turns it round at the ends, so a
// late wake shows as a moment's rest at the wall, never a stutter.
const (
	StartDotSpeed = 2   // pixels per field: whole pixels keep the motion even on the tube
	StartDotPath  = 80  // pixels from one end to the other
	StartDotY     = 240 // the middle of the frame
)

type startDot struct {
	v, xmin, xmax, y int
	vx               int // the run last sent: +v or -v
	fields           int // fields stepped so far
}

// newStartDot lays the path out from a "speed,path" spec (PLEXCRT_START_DOT,
// for tuning on a set); an empty or unreadable part keeps its default.
func newStartDot(spec string) *startDot {
	v, path := StartDotSpeed, StartDotPath
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
	return &startDot{v: v, xmin: xmin, xmax: xmin + path, y: StartDotY, vx: v}
}

// step runs once a field: the dot shows, and turns round at an end.
func (d *startDot) step(osd OSD) {
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

// stop hides the dot and ends its run.
func (d *startDot) stop(osd OSD) {
	osd.Dot(0, 0, 0, false)
	osd.DotRun(0, SafeX, SafeX+SafeW)
}
