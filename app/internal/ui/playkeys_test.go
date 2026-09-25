package ui

import (
	"testing"
	"time"

	"plexcrt/internal/input"
)

// keyAt presses a key on the controller at a time and lets the overlay
// answer at once, as playLoop does; true when playback should stop.
func keyAt(p *Playing, c *coreOSD, k input.Key, at time.Time) bool {
	if p.Key(input.Event{Key: k}, at) {
		return true
	}
	p.Tick(at, c, false)
	return false
}

func TestBackRestsAfterTheOverlayGoes(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	p.Tick(now, c, true)
	keyAt(p, c, input.Enter, now) // the controls come up
	if !p.visible {
		t.Fatal("OK did not open the controls")
	}
	now = now.Add(time.Second)
	if keyAt(p, c, input.Back, now) || p.visible {
		t.Fatal("Back did not just close the controls")
	}
	if keyAt(p, c, input.Back, now.Add(300*time.Millisecond)) {
		t.Fatal("a second Back right after the controls closed stopped the playback")
	}
	if !keyAt(p, c, input.Back, now.Add(BackGrace+100*time.Millisecond)) {
		t.Fatal("Back did not stop the playback once the grace was over")
	}
}

func TestBackRestsAfterTheOverlayTimesOut(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	keyAt(p, c, input.Enter, now)
	now = now.Add(OsdHide + 100*time.Millisecond)
	p.Tick(now, c, true) // it hides itself
	if p.visible {
		t.Fatal("the controls did not time out")
	}
	if keyAt(p, c, input.Back, now.Add(200*time.Millisecond)) {
		t.Fatal("Back just as the controls timed out stopped the playback")
	}
	// a press long after anything was on screen stops at once
	if !keyAt(p, c, input.Back, now.Add(5*time.Second)) {
		t.Fatal("Back with nothing on screen did not stop the playback")
	}
}

func TestBackRestsAfterTheSeekStripGoes(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	keyAt(p, c, input.Right, now) // a tap: the strip peeks
	now = now.Add(50 * time.Millisecond)
	p.Key(input.Event{Key: input.Right, Release: true}, now)
	// the seek goes after its grace, then the strip runs out
	for f := 0; ; f++ {
		if f > 600 {
			t.Fatal("the strip never went")
		}
		c.vsync()
		p.Tick(now, c, true)
		if !p.armed && !p.peeking(now) && p.last == "" {
			break
		}
		now = now.Add(16683 * time.Microsecond)
	}
	if keyAt(p, c, input.Back, now.Add(100*time.Millisecond)) {
		t.Fatal("Back just as the strip went stopped the playback")
	}
}

func TestDownStepsOutOfTheControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		open  input.Key
		downs int // presses to close: the timeline steps to the buttons first
	}{
		{"on the buttons", input.Enter, 1},
		{"on the timeline", input.Up, 2},
	} {
		p := waitPlaying(t)
		c := &coreOSD{}
		now := time.Now()
		p.framed(now, c)
		keyAt(p, c, tc.open, now)
		if !p.visible {
			t.Fatalf("%s: the controls did not open", tc.name)
		}
		for i := 1; i <= tc.downs; i++ {
			now = now.Add(100 * time.Millisecond)
			if keyAt(p, c, input.Down, now) {
				t.Fatalf("%s: Down stopped the playback", tc.name)
			}
			if i < tc.downs && (!p.visible || p.scrub) {
				t.Fatalf("%s: Down %d should have moved to the buttons (visible %v, scrub %v)", tc.name, i, p.visible, p.scrub)
			}
		}
		if p.visible || p.scrub || p.armed {
			t.Fatalf("%s: Down left the controls up (visible %v, scrub %v)", tc.name, p.visible, p.scrub)
		}
		// hidden, Down does nothing
		if keyAt(p, c, input.Down, now.Add(3*time.Second)) || p.visible {
			t.Fatalf("%s: Down with the controls closed did something", tc.name)
		}
	}
}

func TestDownFromTheTimelineKeepsTheScrubAndBackDropsIt(t *testing.T) {
	for _, tc := range []struct {
		key  input.Key
		want []string
	}{
		{input.Down, []string{"seek 610"}},
		{input.Back, nil},
	} {
		p := waitPlaying(t)
		var sent []string
		p.send = func(s string) { sent = append(sent, s) }
		c := &coreOSD{}
		now := time.Now()
		p.framed(now, c)
		keyAt(p, c, input.Up, now) // the timeline, at 600 s
		keyAt(p, c, input.Right, now.Add(100*time.Millisecond))
		p.Key(input.Event{Key: input.Right, Release: true}, now.Add(150*time.Millisecond)) // a tap: 610, in its grace
		keyAt(p, c, tc.key, now.Add(250*time.Millisecond))
		if !p.visible || p.scrub {
			t.Fatalf("%v from the timeline: visible %v, scrub %v; want the buttons", tc.key, p.visible, p.scrub)
		}
		tickFields(p, c, now.Add(300*time.Millisecond), 60, nil) // well past the grace
		if len(sent) != len(tc.want) || len(sent) == 1 && sent[0] != tc.want[0] {
			t.Fatalf("%v from the timeline in a scrub's grace sent %q, want %q", tc.key, sent, tc.want)
		}
	}
}

func TestDownMovesInAList(t *testing.T) {
	p := waitPlaying(t)
	c := &coreOSD{}
	now := time.Now()
	p.framed(now, c)
	keyAt(p, c, input.Enter, now)
	p.openMore()
	p.openCrop()
	p.list.cur = 0
	keyAt(p, c, input.Down, now.Add(100*time.Millisecond))
	if p.list == nil || !p.visible || p.list.cur != 1 {
		t.Fatalf("Down in a list did not move its cursor: list %+v, visible %v", p.list, p.visible)
	}
}
