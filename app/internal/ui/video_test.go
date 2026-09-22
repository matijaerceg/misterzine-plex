package ui

import (
	"plexcrt/internal/input"
	"testing"
	"time"
)

func TestVideoHoldRequiresFreshContinuousPress(t *testing.T) {
	now := time.Now()
	h := videoHold{}
	h.key(input.Event{Key: input.Enter}, now)
	if h.ready(now.Add(3 * time.Second)) {
		t.Fatal("opening press confirmed mode")
	}
	h.key(input.Event{Key: input.Enter, Release: true}, now)
	for i := 0; i < 5; i++ {
		h.key(input.Event{Key: input.Enter}, now)
		h.key(input.Event{Key: input.Enter, Release: true}, now.Add(400*time.Millisecond))
		now = now.Add(500 * time.Millisecond)
	}
	if h.ready(now) {
		t.Fatal("taps accumulated")
	}
	h.key(input.Event{Key: input.Enter}, now)
	h.key(input.Event{Key: input.Enter, Repeat: true}, now.Add(time.Second))
	if h.ready(now.Add(1999 * time.Millisecond)) {
		t.Fatal("accepted too early")
	}
	if !h.ready(now.Add(2 * time.Second)) {
		t.Fatal("hold did not confirm")
	}
	h.key(input.Event{Key: input.Enter, Release: true}, now.Add(2100*time.Millisecond))
	if h.ready(now.Add(3 * time.Second)) {
		t.Fatal("release did not cancel hold")
	}
}
