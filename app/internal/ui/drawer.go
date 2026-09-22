package ui

import (
	"math"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
)

// Drawer is the menu: a panel that slides in down the left over the home
// screen, which shows dimmed behind it. Home, the libraries, Options.
type Drawer struct {
	app     *App
	items   []*plex.Item
	cur     int
	back    *gfx.Image  // the home screen, dimmed
	page    *gfx.Canvas // the composed drawer over it
	key     string      // what the page was composed for
	openAt  time.Time
	closeAt time.Time // set while sliding out; Pop follows
	then    func()    // runs once the drawer has slid out
}

const (
	DrawerW    = 324
	DrawerRowH = MenuRowH
	DrawerY0   = SafeY + 64
	BackBright = 56 // of 255: the home screen behind the menu
)

// NewDrawer composes the menu over a snapshot of the screen beneath.
func NewDrawer(app *App, under Screen, items []*plex.Item) *Drawer {
	d := &Drawer{app: app, items: items, openAt: time.Now()}
	snap := gfx.NewCanvas(720, 480)
	under.Draw(snap, d.openAt)
	d.back = (&gfx.Image{W: snap.W, H: snap.H, Pix: snap.Pix}).Dimmed(BackBright)
	d.page = gfx.NewCanvas(720, 480)
	return d
}

// Close slides the drawer out, then runs then (after the Pop).
func (d *Drawer) Close(then func()) {
	if !d.closeAt.IsZero() {
		return
	}
	d.closeAt = time.Now()
	d.then = then
}

// Back slides the drawer out.
func (d *Drawer) Back() bool {
	d.Close(nil)
	return true
}

// Key handles one input event: Up/Down choose, OK enters, Left/Right close.
func (d *Drawer) Key(ev input.Event, now time.Time) {
	if ev.Release || !d.closeAt.IsZero() {
		return
	}
	switch ev.Key {
	case input.Up:
		if d.cur > 0 {
			d.cur--
		}
	case input.Down:
		if d.cur < len(d.items)-1 {
			d.cur++
		}
	case input.Left, input.Right:
		if !ev.Repeat {
			d.Close(nil)
		}
	case input.Enter:
		d.app.MenuPick(d, d.items[d.cur])
	}
}

// offset is how far the panel still sits off the left edge.
func (d *Drawer) offset(now time.Time) (int, bool) {
	var p float64
	moving := false
	if !d.closeAt.IsZero() {
		p = float64(now.Sub(d.closeAt)) / float64(animDur)
		if p >= 1 {
			return -DrawerW, false
		}
		moving = true
		p = 1 - math.Pow(p, 3) // ease in going out
	} else {
		p = float64(now.Sub(d.openAt)) / float64(animDur)
		if p >= 1 {
			p = 1
		} else {
			moving = true
		}
		p = 1 - math.Pow(1-p, 3) // ease out coming in
	}
	return -DrawerW + int(float64(DrawerW)*p+0.5), moving
}

// Draw copies the composed page in, recomposing it when anything moved.
func (d *Drawer) Draw(c *gfx.Canvas, now time.Time) bool {
	off, moving := d.offset(now)
	if !d.closeAt.IsZero() && !moving {
		// slid out: leave; the screen beneath draws the next frame
		then := d.then
		d.closeAt = time.Time{}
		d.app.Pop()
		if then != nil {
			then()
		}
		return d.app.top().Draw(c, now)
	}
	key := itoa(off) + "|" + itoa(d.cur)
	if key != d.key {
		d.compose(off)
		d.key = key
	}
	c.Copy(d.page)
	return moving
}

func (d *Drawer) compose(off int) {
	c := d.page
	c.Blit(0, 0, d.back)
	c.Fill(off, 0, DrawerW, c.H, gfx.Bg)
	d.app.Mark.Place(c, off+MenuX, SafeY-8)
	f := d.app.F.Body
	y := DrawerY0
	for i, it := range d.items {
		col := gfx.GreyHi
		if i == d.cur {
			col = gfx.White
			menuFocusBar(c, off+MenuX, y, f.Height())
		}
		c.Text(off+MenuX, y, f, col, f.Fit(it.Title, DrawerW-MenuX-24))
		y += DrawerRowH
	}
}
