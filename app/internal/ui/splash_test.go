package ui

import (
	"testing"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
)

type splashTestScreen struct{}

func (*splashTestScreen) Key(input.Event, time.Time) {}
func (*splashTestScreen) Draw(c *gfx.Canvas, _ time.Time) bool {
	c.Fill(0, 0, c.W, c.H, 0x204060)
	return false
}

func TestFirstFrameIsTheScreenItself(t *testing.T) {
	a := &App{stack: []Screen{&splashTestScreen{}}}
	c := gfx.NewCanvas(720, 480)
	if a.drawScreen(c, time.Now()) {
		t.Fatal("idle screen reported as animating")
	}
	clean := gfx.NewCanvas(c.W, c.H)
	a.top().Draw(clean, time.Now())
	for i := range c.Pix {
		if c.Pix[i] != clean.Pix[i] {
			t.Fatal("startup drew something over the first screen")
		}
	}
}
