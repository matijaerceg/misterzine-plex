package ui

import (
	"math"
	"time"
)

// Anim eases a value from where it is to a target over a fixed duration.
// Retargeting mid-flight starts from the current eased value, so quick key
// repeats never snap.
type Anim struct {
	from, to float64
	start    time.Time
	dur      time.Duration
	linear   bool
}

const animDur = 160 * time.Millisecond

// RepeatDur is the auto-repeat cadence. A held key moves one step per
// repeat, linearly, over a segment a quarter longer than the cadence: the
// segments overlap, so a repeat arriving a few ms late never stalls the
// motion at a row boundary, and the speed settles to one step per repeat.
const RepeatDur = 140 * time.Millisecond
const glideDur = RepeatDur * 5 / 4

// Set jumps without animating.
func (a *Anim) Set(v float64) { a.from, a.to, a.dur = v, v, 0 }

// Go retargets, starting from the current value, easing out.
func (a *Anim) Go(v float64, now time.Time) { a.retarget(v, now, animDur, false) }

// Glide retargets linearly (for held keys).
func (a *Anim) Glide(v float64, now time.Time) { a.retarget(v, now, glideDur, true) }

// Settle finishes the current motion with an ease-out (on key release).
func (a *Anim) Settle(now time.Time) {
	if !a.Running(now) || !a.linear {
		return
	}
	a.from = a.At(now)
	a.start = now
	a.dur = animDur
	a.linear = false
}

// Move is Go or Glide depending on whether the key is auto-repeating.
func (a *Anim) Move(v float64, now time.Time, repeat bool) {
	if repeat {
		a.Glide(v, now)
	} else {
		a.Go(v, now)
	}
}

func (a *Anim) retarget(v float64, now time.Time, dur time.Duration, linear bool) {
	if v == a.to {
		return
	}
	a.from = a.At(now)
	a.to = v
	a.start = now
	a.dur = dur
	a.linear = linear
}

// At returns the eased value at now.
func (a *Anim) At(now time.Time) float64 {
	if a.dur == 0 {
		return a.to
	}
	p := float64(now.Sub(a.start)) / float64(a.dur)
	if p >= 1 {
		return a.to
	}
	if p < 0 {
		p = 0
	}
	if !a.linear {
		p = 1 - math.Pow(1-p, 3) // ease-out cubic
	}
	return a.from + (a.to-a.from)*p
}

// Running reports whether the animation is still moving at now.
func (a *Anim) Running(now time.Time) bool {
	return a.dur != 0 && now.Sub(a.start) < a.dur
}

// Target is where the animation is heading.
func (a *Anim) Target() float64 { return a.to }

func round(v float64) int { return int(math.Floor(v + 0.5)) }
