package ui

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
	"plexcrt/internal/ring"
)

// Playing drives the overlay while a video plays: OK opens it, Back
// closes it (and stops when it is closed), left and right seek ten
// seconds with it closed (a slim peek of the bar and the times shows the
// jump) and move along its buttons with it open. It hides itself after a
// few seconds unless paused. Audio and Subtitles
// open a list in the overlay; a skip button appears over an intro or
// credits marker; at the end a countdown runs on to the next episode.
type Playing struct {
	app     *App
	item    *plex.Item
	queue   []*plex.Item // the episodes around it, for Prev and Next
	idx     int
	send    func(string)
	visible bool
	shownAt time.Time
	peekAt  time.Time     // a closed-overlay seek: the bar and times peek until OsdFlash
	peekH   int           // the peek's height, from the last compose
	peekFor time.Duration // how long the peek stays after peekAt; 0 means OsdFlash
	focus   int
	paused  bool
	pos     float64
	dur     float64
	statAt  time.Time // when the launcher's status was last read
	seekAt  time.Time // a seek was sent: the position jumps once the stream restarts
	seekTo  float64
	canvas  *gfx.Canvas
	alpha   []byte
	dirty   bool
	last    string // what the overlay last showed, to skip identical redraws
	// the sprites the core draws: set by compose, sent by Tick
	dotX, dotY int
	dotOn      bool
	barX, barY int
	barW       int
	barH       int
	barOn      bool
	btnX, btnW [8]int // button places from the last compose, for the bar sprite
	btnY       int
	pillY      int
	composed   bool
	painter    *painter
	Next       int // set to +1 or -1 when Prev or Next was chosen; the queue continues

	list     *listMode // an open audio or subtitle list
	scrub    bool      // the cursor is on the timeline
	scrubTo  float64   // where it points
	held     int       // -1/+1 while left or right is held in scrub mode
	settle   bool      // a run just ended: take the dot's place from the core
	pausedAt time.Time // when Pause was last pressed
	runVx    int       // the run speed last sent to the core
	heldAt   time.Time // since when
	skip     *plex.Marker
	skipOff  bool // the skip button was dismissed for this marker
	skipAt   time.Time
	restart  bool // the stream must restart for a new track: seek to here
}

type listMode struct {
	kind  string
	items []string
	cur   int
	set   int
	off   int // subtitles: item 0 is Off
	// The list is drawn once as a tall column in the overlay region and
	// the core is pointed at a window of it: a scroll is a header store.
	// at/to are the window's top in column pixels; base is the column
	// row the upload starts at (a very long list is uploaded in parts).
	at, to   int
	base     int
	loaded   bool
	uploaded int // rows uploaded from base
}

// The list column.
const (
	OsdListW    = 360                // column width; 1440-byte rows
	OsdListH    = SafeBottom - SafeY // the window: from the safe top down
	OsdListTop  = 56                 // the first row, below the title
	OsdListRowH = MenuRowH
	OsdListPad  = 16
)

// height is the column's height in pixels.
func (l *listMode) height() int { return OsdListTop + len(l.items)*OsdListRowH + OsdListPad }

// window aims the scroll so the cursor's row is inside it: scrolling up
// to the first item brings the title back.
func (l *listMode) window() {
	top := l.cur * OsdListRowH // the title above item 0 stays until then
	if l.cur == 0 {
		top = 0
	}
	if top < l.to {
		l.to = top
	}
	if bottom := OsdListTop + (l.cur+1)*OsdListRowH; bottom > l.to+OsdListH {
		l.to = bottom - OsdListH
	}
	l.to = max(0, min(l.to, l.height()-OsdListH))
}

// slide moves the window toward its aim in even steps that ease out:
// 12, 10, 6, 4 pixels over four fields for one row.
func (l *listMode) slide() bool {
	d := l.to - l.at
	if d == 0 {
		return false
	}
	step := (abs(d)/2 + 1) &^ 1
	step = max(4, min(step, 12))
	step = min(step, abs(d))
	if d < 0 {
		step = -step
	}
	l.at += step
	return true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

const (
	OsdY      = 296 // controls stay in the safe area; background reaches the raster edge
	OsdH      = 480 - OsdY
	OsdAlpha  = 200
	OsdHide   = 4 * time.Second
	OsdFlash  = 2 * time.Second         // how long a closed-overlay seek shows the bar
	OsdLinger = 3500 * time.Millisecond // how long the start strip stays over the picture
	OsdBtnGap = 30
	SkipHold  = 8 * time.Second // the skip button stays this long after the marker starts
)

var osdButtons = []string{"-10", "Pause", "+10", "Prev", "Next", "Audio", "Subs"}

// NewPlaying makes the controller for an item; send writes to the launcher's FIFO.
func NewPlaying(app *App, it *plex.Item, queue []*plex.Item, idx int, send func(string)) *Playing {
	p := &Playing{app: app, item: it, queue: queue, idx: idx, send: send, focus: 1}
	p.dur = float64(it.Duration)
	p.canvas = gfx.NewCanvas(720, 480)
	p.alpha = make([]byte, 720*480)
	p.painter = newPainter()
	if len(it.Markers) == 0 || it.PartID == "" {
		if fresh, err := app.Plex.Item(it.RatingKey); err == nil {
			it.Markers, it.PartID, it.Audio, it.Subs = fresh.Markers, fresh.PartID, fresh.Audio, fresh.Subs
		}
	}
	return p
}

// status reads the launcher's position file.
func (p *Playing) status() {
	data, err := os.ReadFile("/tmp/plexplay.stat")
	if err != nil {
		return
	}
	var st struct {
		Pos    float64 `json:"pos"`
		Dur    float64 `json:"dur"`
		Paused bool    `json:"paused"`
	}
	if json.Unmarshal(data, &st) != nil {
		return
	}
	if !p.seekAt.IsZero() {
		// until the stream restarts at the new place, show where we asked for
		// a report from before the restart is still the old place; ten-second
		// steps must not be swallowed by it, so the window is tight
		if time.Since(p.seekAt) < 3*time.Second && (st.Pos < p.seekTo-4 || st.Pos > p.seekTo+4) {
			return
		}
		p.seekAt = time.Time{}
	}
	p.pos = st.Pos
	if time.Since(p.pausedAt) > 700*time.Millisecond {
		p.paused = st.Paused
	}
	if st.Dur > 0 {
		p.dur = st.Dur
	}
}

func (p *Playing) show(now time.Time) {
	p.visible = true
	p.shownAt = now
	p.focus = 1 // Pause, every time
	p.scrub = false
	p.dirty = true
}

// ScrubHold is how long a direction is held before the cursor starts to
// run; a shorter press is one ten-second step.
const ScrubHold = 220 * time.Millisecond

// running reports whether a held direction has passed the tap threshold:
// the core is then moving the dot on its own, field-locked.
func (p *Playing) running(now time.Time) bool {
	return p.scrub && p.held != 0 && now.Sub(p.heldAt) >= ScrubHold && p.dur > 0
}

// runSpeed is the hold's pixels per field: one at first, three after a
// second. Whole pixels per field keep the motion even on the tube.
func (p *Playing) runSpeed(now time.Time) int {
	hold := now.Sub(p.heldAt).Seconds() - ScrubHold.Seconds()
	return min(1+int(hold*2), 3)
}

// fromX is the film time the dot at frame column x stands for.
func (p *Playing) fromX(x int) float64 {
	return max(0, min(float64(x-SafeX)*p.dur/float64(SafeW), p.dur-6))
}

func (p *Playing) seekBy(d float64) {
	p.seekTo = max(0, min(p.pos+d, p.dur-6))
	p.pos = p.seekTo
	p.seekAt = time.Now()
	p.send("seek " + ftoa(p.seekTo))
	p.dirty = true
}

func (p *Playing) seekTo_(t float64) {
	p.seekTo = max(0, min(t, p.dur-6))
	p.pos = p.seekTo
	p.seekAt = time.Now()
	p.send("seek " + ftoa(p.seekTo))
	p.dirty = true
}

func ftoa(f float64) string { return itoa(int(f)) }

// marker is the intro or credits marker the position is inside, if any.
func (p *Playing) marker() *plex.Marker {
	for i := range p.item.Markers {
		m := &p.item.Markers[i]
		if p.pos >= m.Start && p.pos < m.End-2 {
			return m
		}
	}
	return nil
}

func (p *Playing) openList(kind string) {
	streams := p.item.Audio
	off := 0
	if kind == "Subtitles" {
		streams = p.item.Subs
		off = 1
	}
	l := &listMode{kind: kind, off: off, set: -1}
	if off == 1 {
		l.items = append(l.items, "Off")
		l.set = 0
	}
	for i, s := range streams {
		l.items = append(l.items, s.Title)
		if s.Selected {
			l.set = i + off
		}
	}
	l.cur = max(l.set, 0)
	l.window()
	l.at = l.to // open on the cursor, no slide
	p.list = l
	p.dirty = true
}

// Key handles a pad event during playback; returns true when playback
// should stop (or move along the queue: Next is set).
func (p *Playing) Key(ev input.Event, now time.Time) bool {
	if ev.Release {
		if ev.Key == input.Left || ev.Key == input.Right {
			if p.running(now) {
				p.settle = true // the core has the dot: adopt its place next field
			}
			p.held = 0
		}
		return false
	}
	if l := p.list; l != nil {
		switch ev.Key {
		case input.Up:
			if l.cur > 0 {
				l.cur--
			}
			l.window()
		case input.Down:
			if l.cur < len(l.items)-1 {
				l.cur++
			}
			l.window()
		case input.Back, input.Left, input.Right:
			if !ev.Repeat {
				p.list = nil
			}
		case input.Enter:
			if !ev.Repeat {
				p.app.selectStream(p.item, l.kind, l.cur-l.off)
				p.list = nil
				// the transcoder starts over with the new track from here
				p.seekTo_(p.pos)
			}
		}
		p.shownAt = now
		p.dirty = true
		return false
	}
	if p.scrub && p.visible {
		switch ev.Key {
		case input.Left, input.Right:
			if ev.Repeat {
				break // the hold runs the cursor from Tick, not the key repeat
			}
			d := 1
			if ev.Key == input.Left {
				d = -1
			}
			p.scrubTo = max(0, min(p.scrubTo+float64(10*d), p.dur-6))
			p.held, p.heldAt = d, now
		case input.Enter:
			if !ev.Repeat {
				if (p.running(now) || p.settle) && p.app.osd != nil {
					p.scrubTo = p.fromX(p.app.osd.DotX())
				}
				p.seekTo_(p.scrubTo)
			}
		case input.Down, input.Back:
			if !ev.Repeat {
				p.scrub = false
			}
		case input.Up:
		}
		p.visible = true // scrubbing keeps the overlay up, even past its timeout
		p.shownAt = now
		p.dirty = true
		return false
	}
	switch ev.Key {
	case input.Up:
		if ev.Repeat {
			return false
		}
		if !p.visible {
			p.visible = true
		}
		p.scrub = true
		p.scrubTo = p.pos
		p.held = 0
		p.shownAt = now
		p.dirty = true
	case input.Enter:
		if ev.Repeat {
			return false
		}
		if m := p.marker(); m != nil && !p.skipOff && !p.visible {
			p.seekTo_(m.End) // the skip button
			p.skipOff = true
			return false
		}
		if !p.visible {
			p.show(now)
			return false
		}
		p.shownAt = now
		switch osdButtons[p.focus] {
		case "-10":
			p.seekBy(-10)
		case "+10":
			p.seekBy(10)
		case "Pause":
			p.paused = !p.paused
			p.pausedAt = now // the launcher's report lags: believed again once it caught up
			p.send("toggle")
			p.dirty = true
		case "Prev":
			if p.idx > 0 {
				p.Next = -1
				return true
			}
		case "Next":
			if p.idx+1 < len(p.queue) {
				p.Next = 1
				return true
			}
		case "Audio":
			if len(p.item.Audio) > 1 {
				p.openList("Audio")
			}
		case "Subs":
			if len(p.item.Subs) > 0 {
				p.openList("Subtitles")
			}
		}
	case input.Left, input.Right:
		d := 1
		if ev.Key == input.Left {
			d = -1
		}
		if p.visible {
			for i := 0; i < len(osdButtons); i++ {
				p.focus = (p.focus + d + len(osdButtons)) % len(osdButtons)
				if !p.dimmed(p.focus) {
					break
				}
			}
			p.shownAt = now
			p.dirty = true
		} else {
			p.seekBy(float64(10 * d))
			p.peekAt, p.peekFor = now, 0
			p.dirty = true
		}
	case input.Back:
		if ev.Repeat {
			return false
		}
		if p.peeking(now) {
			p.peekAt = time.Time{}
			p.dirty = true
			return false
		}
		if m := p.marker(); m != nil && !p.skipOff && !p.visible {
			p.skipOff = true // dismiss the skip button
			p.dirty = true
			return false
		}
		if p.visible {
			p.visible = false
			p.scrub = false
			p.dirty = true
			return false
		}
		return true
	}
	return false
}

// Tick refreshes the position and redraws the overlay when needed. It
// runs once per field (field true) and right after a key (field false).
func (p *Playing) Tick(now time.Time, osd OSD, field bool) {
	if now.Sub(p.statAt) > 100*time.Millisecond {
		p.statAt = now
		p.status()
	}
	if field && p.scrub && p.held != 0 {
		p.shownAt = now // a hold keeps the overlay up
	}
	// the dot's run is the core's: send the speed, follow its place
	vx := 0
	if p.running(now) {
		vx = p.held * p.runSpeed(now)
	}
	if vx != p.runVx {
		osd.DotRun(vx, SafeX, SafeX+SafeW)
		p.runVx = vx
	}
	if vx != 0 || p.settle {
		p.scrubTo = p.fromX(osd.DotX())
		if vx == 0 {
			p.settle = false
		}
	}
	if m := p.marker(); m != p.skip {
		p.skip = m
		p.skipOff = false
		p.skipAt = now
		p.dirty = true
	}
	if p.visible && !p.paused && p.list == nil && now.Sub(p.shownAt) > OsdHide {
		p.visible = false
		p.scrub, p.held = false, 0
		p.dirty = true
	}
	if !p.peekAt.IsZero() && !p.peeking(now) {
		p.peekAt = time.Time{}
		p.dirty = true
	}
	peek := p.peeking(now)
	skipBtn := p.skip != nil && !p.skipOff && now.Sub(p.skipAt) < SkipHold && !p.visible && !peek
	if !p.visible && !skipBtn && !peek {
		if p.dirty {
			p.painter.hide(osd)
			osd.Dot(0, 0, 0, false)
			osd.Bar(0, 0, 0, 0, 0, false)
			p.dirty = false
			p.last = ""
		}
		return
	}
	var key string
	var x, y int
	if p.list != nil {
		l := p.list
		osd.Dot(0, 0, 0, false)
		moved := false
		if field {
			moved = l.slide()
		}
		// the window must lie within what is uploaded
		if !l.loaded || l.at < l.base || l.at+OsdListH > l.base+l.uploaded {
			p.uploadList(osd)
			moved = true
		}
		if moved || p.last != "list" {
			p.last = "list"
			off := (l.at - l.base) * OsdListW * 4
			p.painter.do(func() { osd.ShowAt(0, SafeY, OsdListW, OsdListH, off) })
		}
		// the cursor is the core's bar sprite, beside its row
		by := SafeY + OsdListTop + l.cur*OsdListRowH - l.at
		osd.Bar(MenuX-MenuBarGap-BarW, by+2, BarW, p.app.F.Body.Height()-4, uint32(gfx.GreyHi),
			by >= SafeY && by+OsdListRowH <= SafeY+OsdListH)
		return
	}
	if peek {
		// the title, the bar and the times, along the bottom edge: no cursor
		osd.Dot(0, 0, 0, false)
		osd.Bar(0, 0, 0, 0, 0, false)
		key = "peek|" + itoa(int(p.pos))
		if key != p.last {
			p.last = key
			p.compose(true)
			h := p.peekH
			p.painter.show(osd, 0, 480-h, &gfx.Canvas{W: 720, H: h, Pix: p.canvas.Pix[:720*h*4]}, p.alpha[:720*h])
		}
		p.dirty = false
		return
	}
	if skipBtn {
		osd.Dot(0, 0, 0, false)
		osd.Bar(0, 0, 0, 0, 0, false)
		x, y = 440, 240
		key = p.skipKey()
		if key != p.last {
			p.last = key
			p.composeSkip()
			p.painter.show(osd, x, y, &gfx.Canvas{W: 240, H: 40, Pix: p.canvas.Pix[:240*40*4]}, p.alpha[:240*40])
		}
		return
	}
	// the sprites first, before any pixel work: one store each, on
	// screen within a line
	if p.composed {
		p.sprites()
		if vx == 0 {
			osd.Dot(p.dotX, p.dotY, uint32(gfx.White), p.dotOn)
		}
		osd.Bar(p.barX, p.barY, p.barW, BarW, uint32(gfx.GreyHi), p.barOn)
	}
	key = p.panelKey()
	if key == p.last && p.composed {
		return // nothing to repaint: composing every field cost a third of a core
	}
	p.compose(false)
	if !p.composed {
		p.composed = true
		p.sprites()
		osd.Dot(p.dotX, p.dotY, uint32(gfx.White), p.dotOn)
		osd.Bar(p.barX, p.barY, p.barW, BarW, uint32(gfx.GreyHi), p.barOn)
	}
	p.last = key
	p.painter.show(osd, 0, OsdY, &gfx.Canvas{W: 720, H: OsdH, Pix: p.canvas.Pix[:720*OsdH*4]}, p.alpha[:720*OsdH])
	p.dirty = false
}

// sprites places the dot and the bar from the current state and the
// geometry the last compose left behind.
func (p *Playing) sprites() {
	p.dotOn = p.scrub && p.dur > 0
	if p.dotOn {
		p.dotX = SafeX + int(float64(SafeW)*p.scrubTo/p.dur+0.5)
		p.dotY = OsdY + p.pillY + PillH/2
	}
	p.barOn = !p.scrub
	if p.barOn {
		p.barX, p.barY, p.barW = p.btnX[p.focus], OsdY+p.btnY, p.btnW[p.focus]
	}
}

// dimmed reports whether button i has nothing to do for this item.
func (p *Playing) dimmed(i int) bool {
	switch osdButtons[i] {
	case "Prev":
		return p.idx == 0
	case "Next":
		return p.idx+1 >= len(p.queue)
	case "Audio":
		return len(p.item.Audio) < 2
	case "Subs":
		return len(p.item.Subs) == 0
	}
	return false
}

// panelKey describes what the panel would show, without drawing it.
func (p *Playing) panelKey() string {
	it := p.item
	title, sub := it.Title, ""
	if it.Type == "episode" {
		title = it.GrandTitle
		sub = "S" + itoa(it.Parent) + " E" + itoa(it.Index) + "  " + it.Title
	}
	return title + sub + "|" + itoa(int(p.pos)) + "|" + itoa(p.focus) + "|" + itoa(btoi(p.paused)) + "|" + itoa(btoi(p.scrub)) + "|" + itoa(int(p.scrubTo))
}

// skipKey describes the skip button.
func (p *Playing) skipKey() string {
	if p.skip != nil && strings.Contains(p.skip.Type, "credit") {
		return "skip|Skip credits"
	}
	return "skip|Skip intro"
}

// peeking reports whether a closed-overlay seek is still showing its bar.
func (p *Playing) peeking(now time.Time) bool {
	stay := p.peekFor
	if stay == 0 {
		stay = OsdFlash
	}
	return !p.peekAt.IsZero() && now.Sub(p.peekAt) < stay && !p.visible && p.list == nil
}

// compose paints the panel; for a peek only the title, the bar and the
// times, and peekH is set to the rows they take.
func (p *Playing) compose(peek bool) {
	c := &gfx.Canvas{W: 720, H: OsdH, Pix: p.canvas.Pix[:720*OsdH*4]}
	f := p.app.F
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	// alpha: the panel, its top edge eased in
	for y := 0; y < c.H; y++ {
		a := OsdAlpha
		if y < 20 {
			a = OsdAlpha * y / 20
		}
		row := p.alpha[y*c.W : (y+1)*c.W]
		for x := range row {
			row[x] = byte(a)
		}
	}
	it := p.item
	title := it.Title
	sub := ""
	if it.Type == "episode" {
		title = it.GrandTitle
		sub = "S" + itoa(it.Parent) + " E" + itoa(it.Index) + "  " + it.Title
	}
	y := 14
	{
		p.app.text(c, SafeX, y, f.Body, gfx.GreyHi, f.Body.Fit(title, SafeW-320))
		if sub != "" {
			p.app.textRight(c, SafeX+SafeW, y+3, f.SmallBold, gfx.GreyLo, f.SmallBold.Fit(sub, 300))
		}
		y += f.Body.Height() + 14
	}
	// the pill, marker spans on it, a dot while scrubbing, and the times:
	// elapsed of total on the left, remaining on the right
	pw := SafeW
	p.pillY = y + 6
	c.Fill(SafeX, y+6, pw, PillH, 0x404040)
	if p.dur > 0 {
		for _, m := range it.Markers {
			x0 := SafeX + int(float64(pw)*m.Start/p.dur)
			x1 := SafeX + int(float64(pw)*m.End/p.dur)
			c.Fill(x0, y+6, max(2, x1-x0), PillH, 0x5A6070)
		}
		c.Fill(SafeX, y+6, int(float64(pw)*p.pos/p.dur), PillH, gfx.Amber)
	}
	y += PillH + 14
	if p.scrub {
		// the time the dot stands for: fixed in the middle, in the body
		// face, so it reads steadily while the dot runs. Changes every
		// field: drawn now, not through the cache.
		c.TextCenter(SafeX+SafeW/2, y-3, f.Body, gfx.White, clock(int(p.scrubTo)))
	} else {
		c.Text(MenuX, y, f.SmallBold, gfx.GreyLo, clock(int(p.pos))+" / "+clock(int(p.dur)))
		if p.dur > 0 {
			c.TextRight(SafeX+SafeW, y, f.SmallBold, gfx.GreyLo, "-"+clock(int(p.dur-p.pos)))
		}
	}
	y += f.SmallBold.Height() + 12
	if peek {
		p.peekH = y
		return
	}
	// the buttons, centred, the focused one white with a bar under it
	labels := make([]string, len(osdButtons))
	total := 0
	for i, b := range osdButtons {
		if b == "Pause" && p.paused {
			b = "Play"
		}
		labels[i] = b
		total += f.Body.Width(b)
	}
	total += OsdBtnGap * (len(labels) - 1)
	x := (c.W - total) / 2
	for i, b := range labels {
		col := gfx.GreyLo
		if b == "Play" {
			col = gfx.Amber
		}
		if i == p.focus && !p.scrub {
			if col != gfx.Amber {
				col = gfx.White
			}
		}
		p.btnX[i], p.btnW[i], p.btnY = x, f.Body.Width(b), y+f.Body.Height()+3
		if p.dimmed(i) {
			col = ChevronOff
		}
		c.Text(x, y, f.Body, col, b) // a colour change on focus would miss the cache for a frame
		x += f.Body.Width(b) + OsdBtnGap
	}
}

// composeSkip paints the small skip button (240x40) at the canvas origin.
func (p *Playing) composeSkip() {
	c := &gfx.Canvas{W: 240, H: 40, Pix: p.canvas.Pix[:240*40*4]}
	f := p.app.F
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	for i := range p.alpha[:240*40] {
		p.alpha[i] = OsdAlpha
	}
	label := "Skip intro"
	if p.skip != nil && strings.Contains(p.skip.Type, "credit") {
		label = "Skip credits"
	}
	c.Text(24, 8, f.Body, gfx.White, label)
	chevronRight(c, 24+f.Body.Width(label)+18, 14, gfx.GreyLo)

}

// composeList paints an audio or subtitle list down the left, full height.
// uploadList draws the part of the column around the window into the
// overlay region: the whole list when it fits (the usual case), else a
// stretch with headroom either way, redone when the window leaves it.
func (p *Playing) uploadList(osd OSD) {
	l := p.list
	f := p.app.F
	maxRows := (ring.RegionSize / (OsdListW * 4)) &^ 1
	h := l.height()
	base := 0
	if h > maxRows {
		base = max(0, min(l.at-(maxRows-OsdListH)/2, h-maxRows)) &^ 1
	}
	// at least the window's height: what the core shows below a short
	// list must be this list's background, not what was there before
	rows := max(min(h-base, maxRows), OsdListH)
	c := gfx.NewCanvas(OsdListW, rows)
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	if base == 0 {
		c.Text(MenuX, 8, f.Title, gfx.Grey, l.kind)
	}
	for i, item := range l.items {
		y := OsdListTop + i*OsdListRowH - base
		if y+OsdListRowH <= 0 || y >= rows {
			continue
		}
		c.Text(MenuX, y, f.Body, gfx.GreyHi, f.Body.Fit(item, OsdListW-MenuX-52))
		if i == l.set {
			tick(c, OsdListW-30, y+f.Body.Height()/2, gfx.GreyHi)
		}
	}
	l.base, l.uploaded, l.loaded = base, rows, true
	p.painter.do(func() { osd.Upload(0, c, 240) })
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
