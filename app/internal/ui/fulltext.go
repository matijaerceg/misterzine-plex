package ui

import (
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
)

// FullText shows a long text (a synopsis) in full, scrolling by line.
type FullText struct {
	app   *App
	title string
	lines []string
	top   Anim
}

const TextLineH = 26

// NewFullText wraps the text for the body font.
func NewFullText(app *App, title, text string) *FullText {
	return &FullText{app: app, title: title, lines: wrapAll(app.F.Body, text, SafeW)}
}

func (t *FullText) rows() int { return (SafeBottom - ListY0) / TextLineH }

// Key scrolls: Up/Down a line, Left/Right or L/R a page.
func (t *FullText) Key(ev input.Event, now time.Time) {
	if ev.Release {
		t.top.Settle(now)
		return
	}
	first := round(t.top.Target())
	last := max(0, len(t.lines)-t.rows())
	switch ev.Key {
	case input.Up:
		first--
	case input.Down:
		first++
	case input.Left, input.JumpBack:
		first -= t.rows()
	case input.Right, input.JumpFwd:
		first += t.rows()
	case input.Enter:
		t.app.Pop()
		return
	}
	first = max(0, min(first, last))
	t.top.Move(float64(first), now, ev.Repeat)
}

// Draw paints the title and the visible lines.
func (t *FullText) Draw(c *gfx.Canvas, now time.Time) bool {
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	f := t.app.F
	off := t.top.At(now)
	for i, line := range t.lines {
		y := ListY0 + round((float64(i)-off)*TextLineH)
		if y+TextLineH < ListY0 || y >= SafeBottom {
			continue
		}
		t.app.textClip(c, SafeX, y, f.Body, gfx.GreyHi, line, ListY0, SafeBottom)
	}
	c.Fill(0, 0, c.W, ListY0-4, gfx.Bg)
	t.app.text(c, SafeX, SafeY, f.Body, gfx.GreyHi, f.Body.Fit(t.title, SafeW))
	return t.top.Running(now)
}
