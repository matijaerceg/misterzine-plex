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

func TestStartupSplashBlendsAndExpires(t *testing.T) {
	a := &App{stack: []Screen{&splashTestScreen{}}, splash: startupSplash{enabled: true}}
	c := gfx.NewCanvas(720, 480)
	now := time.Unix(100, 0)
	if !a.drawScreen(c, now) {
		t.Fatal("splash did not animate")
	}
	full := append([]byte(nil), c.Pix...)
	if a.splash.logo.W != 320 {
		t.Fatal("logo should be twice the original width")
	}
	if !a.drawScreen(c, now.Add(startupFade/2)) {
		t.Fatal("splash stopped early")
	}
	half := append([]byte(nil), c.Pix...)
	if a.drawScreen(c, now.Add(startupFade)) {
		t.Fatal("finished splash kept idle screen animating")
	}
	if a.splash.logo != nil || a.splash.page != nil || a.splash.enabled {
		t.Fatal("splash retained temporary buffers")
	}
	blended := 0
	for i := range c.Pix {
		if i%4 == 3 {
			continue
		}
		want := (int(full[i]) + int(c.Pix[i])) / 2
		if d := int(half[i]) - want; d < -2 || d > 2 {
			t.Fatalf("halfway channel %d is %d, want near %d", i, half[i], want)
		}
		if half[i] != full[i] && half[i] != c.Pix[i] {
			blended++
		}
	}
	if blended == 0 {
		t.Fatal("no intermediate colours")
	}
	clean := gfx.NewCanvas(c.W, c.H)
	a.top().Draw(clean, now)
	for i := range c.Pix {
		if c.Pix[i] != clean.Pix[i] {
			t.Fatal("splash left pixels behind")
		}
	}
}
