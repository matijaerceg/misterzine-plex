// Package input turns the core's joystick and PS/2 status words into key
// events with auto-repeat. MiSTer main holds every evdev node exclusively, so
// this is the only route to the pad while a core runs.
package input

import (
	"sync/atomic"
	"time"
)

// Key is a logical control, in the order of the core's J1 line
// (OK, Back, L, R on joystick bits 4..7). L/R jump sections; everything
// else is the d-pad, OK and Back.
type Key int

const (
	Right Key = iota
	Left
	Down
	Up
	Enter // OK
	Back
	JumpBack // L
	JumpFwd  // R
	None
)

var names = [...]string{"right", "left", "down", "up", "enter", "back", "l", "r", "none"}

func (k Key) String() string { return names[k] }

// Event is one press (or auto-repeat) of a key, or the release that ends a
// held run (Release set, Count repeats delivered).
type Event struct {
	Key      Key
	Keyboard bool // distinguish typing from controller navigation
	ScanCode int  // PS/2 set 2, extended codes have bit 8 set
	Text     rune // printable US-layout character, when present
	Repeat   bool
	Release  bool
	Count    int
}

// Source is anything exposing the two status words.
type Source interface {
	Joy() uint32
	Key() uint32
}

const (
	repeatDelay    = 400 * time.Millisecond
	repeatRate     = 140 * time.Millisecond // keep equal to ui.RepeatDur
	pollPeriod     = 1 * time.Millisecond   // the core posts the pad every field; catch it the ms it lands
	playPollPeriod = 16 * time.Millisecond  // during playback: the decoder needs the core more than the pad does
	backspaceDelay = 250 * time.Millisecond
	backspaceRate  = 45 * time.Millisecond
)

// ps2 set-2 scan codes -> key; extended codes are 0x100|code.
var ps2map = map[int]Key{
	0x175: Up, 0x172: Down, 0x16B: Left, 0x174: Right,
	0x5A: Enter, 0x29: Enter, 0x76: Back, 0x66: Back,
	0x17D: JumpBack, 0x17A: JumpFwd, // PgUp / PgDn
	0x1D: Up, 0x1B: Down, 0x1C: Left, 0x23: Right, // WASD
}

// relaxed is set while video plays: the poll slows to playPollPeriod.
var relaxed atomic.Bool

// Relax slows the poll while video plays (true) and restores it (false).
func Relax(on bool) { relaxed.Store(on) }

// Poll reads the words every few ms and sends events on the returned channel.
func Poll(src Source, stop <-chan struct{}) <-chan Event {
	out := make(chan Event, 64)
	go func() {
		var prevJoy uint32
		var prevKey uint32 = src.Key()
		keyboard := newKeyboard()
		var backspaceNext time.Time
		var held Key = None
		var heldSince, lastRep time.Time
		reps := 0
		t := time.NewTicker(pollPeriod)
		defer t.Stop()
		period := pollPeriod
		for {
			var now time.Time
			select {
			case <-stop:
				close(out)
				return
			case now = <-t.C:
			}
			want := pollPeriod
			if relaxed.Load() {
				want = playPollPeriod
			}
			if want != period {
				t.Reset(want)
				period = want
			}
			j := src.Joy()
			cur := (j | j>>16) & 0xFF // either pad; d-pad, OK, Back, L, R
			pressed := cur &^ ((prevJoy | prevJoy>>16) & 0xFF)
			released := ((prevJoy | prevJoy>>16) & 0xFF) &^ cur
			prevJoy = j
			// OK and Back do not auto-repeat, but holds still need releases.
			for _, k := range []Key{Enter, Back} {
				if released&(1<<k) != 0 {
					out <- Event{Key: k, Release: true}
				}
			}
			for b := 0; b < 8; b++ {
				if pressed&(1<<b) != 0 {
					k := Key(b)
					out <- Event{Key: k}
					if k <= Up || k == JumpBack || k == JumpFwd {
						held, heldSince, lastRep, reps = k, time.Now(), time.Now(), 0
					}
				}
			}
			if held != None {
				if cur&(1<<held) == 0 {
					out <- Event{Key: held, Release: true, Count: reps} // Count 0 for a tap
					held = None
				} else if now.Sub(heldSince) > repeatDelay && now.Sub(lastRep) > repeatRate {
					lastRep = now
					reps++
					out <- Event{Key: held, Repeat: true, Count: reps}
				}
			}
			kw := src.Key()
			if (kw>>11)&0xff != (prevKey>>11)&0xff {
				prevKey = kw
				if ev, ok := keyboard.decode(kw); ok {
					if ev.ScanCode == 0x66 {
						if ev.Release {
							backspaceNext = time.Time{}
						} else if !ev.Repeat {
							backspaceNext = now.Add(backspaceDelay)
						}
						// Own the cadence; hardware typematic must not double it.
						if !ev.Repeat {
							out <- ev
						}
					} else {
						out <- ev
					}
				}
			}
			if !backspaceNext.IsZero() && !now.Before(backspaceNext) {
				out <- Event{Key: Back, Keyboard: true, ScanCode: 0x66, Repeat: true}
				backspaceNext = now.Add(backspaceRate)
			}
		}
	}()
	return out
}
