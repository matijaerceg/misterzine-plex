package ui

import (
	"testing"
	"time"
)

func TestFadeWaitReadyWakesForCompletedStep(t *testing.T) {
	f := &Fade{readyWake: make(chan struct{}, 1), stop: make(chan struct{})}
	f.readyWake <- struct{}{} // a stale notification must not satisfy the wait
	go func() {
		time.Sleep(time.Millisecond)
		f.ready.Store(3)
		f.readyWake <- struct{}{}
	}()
	if got := f.waitReady(3, time.Second); got != 3 {
		t.Fatalf("got step %d before requested step completed", got)
	}
}

func TestFadeWaitReadyStopsAtBudgetOrCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		f := &Fade{readyWake: make(chan struct{}, 1), stop: make(chan struct{})}
		f.ready.Store(1)
		if cancel {
			close(f.stop)
		}
		start := time.Now()
		if got := f.waitReady(2, time.Millisecond); got != 1 {
			t.Fatalf("incomplete step became ready: %d", got)
		}
		if time.Since(start) > 100*time.Millisecond {
			t.Fatal("wait exceeded its bounded budget")
		}
	}
}
