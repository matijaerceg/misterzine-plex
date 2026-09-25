package ui

import (
	"fmt"
	"math"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
)

// Geometry is where video goes on the 720x480 raster, set on the calibration
// screen for sets that hide the picture's edges or draw it too wide or too
// narrow: each edge moved in from the raster's by an even number of pixels
// (left, right) or lines (top, bottom), and the picture's width in thousandths
// of the nominal 4:3 width (0 is 1000). Menus ignore it; the presenter gets it
// as PLEXFB_GEOMETRY and fits every frame inside it.
type Geometry struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Width  int `json:"width,omitempty"`
}

// The limits the presenter accepts (arm/plexfb.c parse_screen).
const (
	GeometryMaxX     = 720 / 6
	GeometryMaxY     = 480 / 6
	GeometryWidthMin = 850
	GeometryWidthMax = 1150
	geometryStep     = 2 // pixels or lines: chroma rows and columns come in pairs, scanlines too
)

// Normal is g within the presenter's limits, edges even, width filled in.
func (g Geometry) Normal() Geometry {
	edge := func(v, hi int) int { return max(0, min(hi, v)) &^ 1 }
	g.Left, g.Right = edge(g.Left, GeometryMaxX), edge(g.Right, GeometryMaxX)
	g.Top, g.Bottom = edge(g.Top, GeometryMaxY), edge(g.Bottom, GeometryMaxY)
	if g.Width == 0 {
		g.Width = 1000
	}
	g.Width = max(GeometryWidthMin, min(GeometryWidthMax, g.Width))
	return g
}

// Env is the PLEXFB_GEOMETRY value.
func (g Geometry) Env() string {
	g = g.Normal()
	return fmt.Sprintf("%d,%d,%d,%d,%d", g.Left, g.Top, g.Right, g.Bottom, g.Width)
}

// Fit is where the presenter puts a frame of display aspect `aspect`: the
// largest rectangle of that shape inside the picture area, centred. It must
// match fit() in arm/plexfb.c; both are tested against
// tools/testdata/fit_cases.txt.
func (g Geometry) Fit(aspect float64) (x, y, w, h int) {
	g = g.Normal()
	aw, ah := 720-g.Left-g.Right, 480-g.Top-g.Bottom
	px := 8000.0 / 9 / float64(g.Width) // a pixel's width, in lines: the raster is 4:3
	if math.Abs(aspect*3/4-1) <= 1.0/60 {
		aspect = 4.0 / 3
	}
	w, h = aw, ah
	if aspect*float64(ah) > float64(aw)*px {
		h = 2 * int(math.Round(float64(aw)*px/aspect/2))
	} else {
		w = 2 * int(math.Round(aspect*float64(ah)/px/2))
	}
	if aw-w <= 2 {
		w = aw
	}
	if ah-h <= 2 {
		h = ah
	}
	w, h = max(w, 16), max(h, 16)
	return g.Left + ((aw-w)/2)&^1, g.Top + ((ah-h)/2)&^1, w, h
}

// squareWidth is the per-mille width that makes a sqW x sqH raster box a
// square on the set: nominally a pixel is 8/9 of a line wide.
func squareWidth(sqW, sqH int) int {
	return int(math.Round(1000 * 8 * float64(sqW) / (9 * float64(sqH))))
}

// VideoGeometry is the presenter's PLEXFB_GEOMETRY, read at each playback start.
func (a *App) VideoGeometry() string { return a.Cfg.Geometry.Env() }

// Calibrate is the video geometry screen: the picture area's edges, and a
// square with a circle in it that a ruler can check. OK steps through the
// four edges and the square's top-right corner, the d-pad moves the one
// selected, Back saves and leaves.
type Calibrate struct {
	app        *App
	g          Geometry
	sel        int
	sqW, sqH   int // the square, in pixels and lines
	sqW0, sqH0 int // its size on opening: the bottom-left corner stays put
}

const (
	calTop = iota
	calRight
	calBottom
	calLeft
	calCorner
	calSteps
)

var calNames = [calSteps]string{"Top edge", "Right edge", "Bottom edge", "Left edge", "Aspect ratio"}

// calFill is the picture area: dark, so the lines and text stand out.
const calFill gfx.Color = 0x1C2230

// NewCalibrate opens the calibration with the saved geometry.
func NewCalibrate(a *App) *Calibrate {
	s := &Calibrate{app: a, g: a.Cfg.Geometry.Normal()}
	// 288 lines (60% of the raster) unless the picture area is too short for
	// it, as wide as the saved width says a square is
	s.sqH = min(288, (480-s.g.Top-s.g.Bottom-60)&^1)
	s.sqW = 2 * int(math.Round(float64(s.sqH)*9/8*float64(s.g.Width)/1000/2))
	s.sqW0, s.sqH0 = s.sqW, s.sqH
	return s
}

// Back saves the geometry and leaves; there is no cancel.
func (s *Calibrate) Back() bool {
	cfg := s.app.Cfg
	before := *cfg
	cfg.Geometry = s.g.Normal()
	if err := cfg.Save(); err != nil {
		s.app.Log.Printf("config: %v", err)
		*cfg = before
		s.app.Notice, s.app.NoticeAt = "Could not save settings. Check free space and retry.", time.Now()
	}
	s.app.Pop()
	return true
}

// Key: OK selects the next edge or the corner; the d-pad moves it. An edge
// moves the way it is pushed, out toward the raster's edge or in toward the
// middle; the corner moves the way it is pushed, and the picture's width
// follows the square's shape.
func (s *Calibrate) Key(ev input.Event, now time.Time) {
	if ev.Release {
		return
	}
	if ev.Key == input.Enter {
		if !ev.Repeat {
			s.sel = (s.sel + 1) % calSteps
		}
		return
	}
	dx := map[input.Key]int{input.Right: 1, input.Left: -1}[ev.Key] * geometryStep
	dy := map[input.Key]int{input.Down: 1, input.Up: -1}[ev.Key] * geometryStep
	g := &s.g
	switch s.sel {
	case calTop:
		g.Top += dy
	case calBottom:
		g.Bottom -= dy
	case calLeft:
		g.Left += dx
	case calRight:
		g.Right -= dx
	case calCorner:
		// the corner stays near where it started, which spans the whole
		// width range (15% of the square is 49 pixels or 43 lines) but
		// keeps the square on the screen when both sides grow
		w, h := s.sqW+dx, s.sqH-dy
		if w < s.sqW0-72 || w > s.sqW0+72 || h < s.sqH0-64 || h > s.sqH0+64 {
			return
		}
		if width := squareWidth(w, h); width >= GeometryWidthMin && width <= GeometryWidthMax {
			s.sqW, s.sqH, g.Width = w, h, width
		}
	}
	*g = g.Normal()
}

// Draw paints the pattern: black outside the picture area, the area's edges
// as lines (the selected one amber), where a 4:3 picture lands filled, and the
// square and circle in the middle with the instructions inside the circle,
// where overscan cannot hide them.
func (s *Calibrate) Draw(c *gfx.Canvas, now time.Time) bool {
	g := s.g.Normal()
	f := s.app.F
	c.Fill(0, 0, c.W, c.H, gfx.Black)
	px, py, pw, ph := g.Fit(4.0 / 3)
	c.Fill(px, py, pw, ph, calFill)

	// the picture area's edges: 2 lines across (1 flickers at 480i), 4
	// pixels down (thinner verticals wash out through composite)
	ax, ay := g.Left, g.Top
	aw, ah := 720-g.Left-g.Right, 480-g.Top-g.Bottom
	col := func(sel int) gfx.Color {
		if s.sel == sel {
			return gfx.Amber
		}
		return gfx.GreyLo
	}
	c.Fill(ax, ay, aw, 2, col(calTop))
	c.Fill(ax, ay+ah-2, aw, 2, col(calBottom))
	c.Fill(ax, ay, 4, ah, col(calLeft))
	c.Fill(ax+aw-4, ay, 4, ah, col(calRight))
	// arrows by the selected edge: it moves either way
	mx, my := ax+aw/2, ay+ah/2
	switch s.sel {
	case calTop:
		chevron(c, mx, ay+10, true, gfx.Amber)
		chevron(c, mx, ay+20, false, gfx.Amber)
	case calBottom:
		chevron(c, mx, ay+ah-26, true, gfx.Amber)
		chevron(c, mx, ay+ah-16, false, gfx.Amber)
	case calLeft:
		chevronLeft(c, ax+16, my-6, gfx.Amber)
		chevronRight(c, ax+28, my-6, gfx.Amber)
	case calRight:
		chevronLeft(c, ax+aw-28, my-6, gfx.Amber)
		chevronRight(c, ax+aw-16, my-6, gfx.Amber)
	}

	// the square grows and shrinks from its bottom-left corner
	sx := mx - s.sqW0/2
	bottom := (my + s.sqH0/2) &^ 1
	sy := bottom - s.sqH
	sq := gfx.GreyHi
	if s.sel == calCorner {
		sq = gfx.Amber
	}
	c.Fill(sx, sy, s.sqW, 2, sq)
	c.Fill(sx, bottom-2, s.sqW, 2, sq)
	c.Fill(sx, sy, 4, s.sqH, sq)
	c.Fill(sx+s.sqW-4, sy, 4, s.sqH, sq)
	cx, cy := sx+s.sqW/2, sy+s.sqH/2
	inside := drawEllipse(c, cx, cy, s.sqW/2, s.sqH/2, 4, 3, sq)
	if s.sel == calCorner {
		cornerArrow(c, sx+s.sqW, sy, gfx.Amber)
	}

	// the instructions, inside the circle
	var lines []string
	value := ""
	if s.sel == calCorner {
		lines = []string{"Use a ruler, and move the", "corner until the square is", "as wide as it is tall"}
		value = fmt.Sprintf("Picture width %d.%d%%", g.Width/10, g.Width%10)
	} else {
		lines = []string{"Line it up with the edge", "of the screen, leaving a", "tiny bit of overscan"}
		n, unit := [...]int{g.Top, g.Right, g.Bottom, g.Left}[s.sel], "pixels"
		if s.sel == calTop || s.sel == calBottom {
			unit = "lines"
		}
		value = itoa(n) + " " + unit + " in"
	}
	ty := cy - 66
	s.centre(c, cx, ty, f.Title, gfx.Amber, calNames[s.sel], inside)
	ty += f.Title.Height() + 8
	for _, l := range lines {
		s.centre(c, cx, ty, f.Body, gfx.GreyHi, l, inside)
		ty += 24
	}
	s.centre(c, cx, ty+6, f.Body, gfx.Amber, value, inside)

	// keys and every value, under the square; what this is, over it
	by := bottom + 12
	if by+f.Small.Height() <= py+ph-8 {
		s.app.textCenterOn(c, mx, by, f.SmallBold, gfx.GreyLo, calFill, "OK: next   Back: save and exit")
	}
	by += 26
	if by+f.Small.Height() <= py+ph-8 {
		all := fmt.Sprintf("Top %d  Right %d  Bottom %d  Left %d  Width %d.%d%%",
			g.Top, g.Right, g.Bottom, g.Left, g.Width/10, g.Width%10)
		s.app.textCenterOn(c, mx, by, f.Small, gfx.GreyLo, calFill, f.Small.Fit(all, pw-24))
	}
	if ty := sy - 30; ty >= py+8 && s.sel != calCorner {
		s.app.textCenterOn(c, mx, ty, f.SmallBold, gfx.GreyLo, calFill, "Video geometry: menus stay as they are")
	}
	return false
}

// centre draws one line centred on cx, cut to the circle's inside at that height.
func (s *Calibrate) centre(c *gfx.Canvas, cx, y int, f *gfx.Font, col gfx.Color, text string, inside func(y0, y1 int) int) {
	s.app.textCenterOn(c, cx, y, f, col, calFill, f.Fit(text, inside(y, y+f.Height())-8))
}

// cornerArrow points down and left at (x, y) from above and to the right:
// a head of two strokes at the tip and a shaft running up at 45 degrees on
// the set (a line is 9/8 of a pixel).
func cornerArrow(c *gfx.Canvas, x, y int, col gfx.Color) {
	tx, ty := x+6, y-8 // the tip, clear of the square's lines
	c.Fill(tx, ty-18, 4, 20, col)
	c.Fill(tx, ty-2, 22, 2, col)
	for i := 0; i < 40; i += 2 {
		c.Fill(tx+2+i*9/8, ty-2-i, 5, 2, col)
	}
}

// drawEllipse strokes the ellipse centred on cx, cy with radii rx, ry, the
// stroke tx pixels wide at the sides and ty lines at the top and bottom. It
// returns the width inside the stroke over a band of rows.
func drawEllipse(c *gfx.Canvas, cx, cy, rx, ry, tx, ty int, col gfx.Color) func(y0, y1 int) int {
	half := func(r, rr int, dy float64) int {
		v := 1 - dy*dy/float64(rr*rr)
		if v <= 0 {
			return -1
		}
		return int(math.Round(float64(r) * math.Sqrt(v)))
	}
	irx, iry := rx-tx, ry-ty
	for yy := cy - ry; yy < cy+ry; yy++ {
		dy := float64(yy) + 0.5 - float64(cy)
		xo := half(rx, ry, dy)
		if xo < 0 {
			continue
		}
		xi := half(irx, iry, dy)
		if xi < 0 {
			c.Fill(cx-xo, yy, 2*xo, 1, col)
			continue
		}
		c.Fill(cx-xo, yy, xo-xi, 1, col)
		c.Fill(cx+xi, yy, xo-xi, 1, col)
	}
	return func(y0, y1 int) int {
		w := 2 * irx
		for yy := y0; yy < y1; yy++ {
			w = min(w, 2*half(irx, iry, float64(yy)+0.5-float64(cy)))
		}
		return max(w, 0)
	}
}
