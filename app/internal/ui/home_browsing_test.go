package ui

import (
	"fmt"
	"testing"
	"time"

	"plexcrt/internal/input"
	"plexcrt/internal/plex"
)

func TestHomeRowMotionDirection(t *testing.T) {
	h := &Home{app: &App{}}
	h.setHubs([]*plex.Hub{homeRow("a", "a"), homeRow("b", "b")})
	now := time.Now()
	h.Key(input.Event{Key: input.Down}, now)
	if h.rowY.At(now) <= 0 || h.rowY.At(now.Add(animDur)) != 0 {
		t.Fatal("down should bring posters up from below and settle")
	}
	h.Key(input.Event{Key: input.Up}, now.Add(animDur))
	if h.rowY.At(now.Add(animDur)) >= 0 {
		t.Fatal("up should bring posters down from above")
	}
	h.Key(input.Event{Key: input.Up}, now.Add(2*animDur))
	if h.rowY.Running(now.Add(2 * animDur)) {
		t.Fatal("edge navigation must not restart row motion")
	}
}

func TestHomeArtworkNeighborhood(t *testing.T) {
	a := NewArt(nil, 0, nil)
	h := &Home{app: &App{Art: a}, home: true}
	var hubs []*plex.Hub
	for r := 0; r < 9; r++ {
		hub := &plex.Hub{}
		for c := 0; c < 30; c++ {
			key := fmt.Sprintf("%d/%d", r, c)
			hub.Items = append(hub.Items, &plex.Item{Thumb: "poster/" + key, Art: "art/" + key})
		}
		hubs = append(hubs, hub)
	}
	h.setHubs(hubs)
	h.row = 4
	h.col[4] = 8
	h.colX[4].Set(6 * PosterPitch)
	a.Get("visible", PosterW, PosterH)
	h.prefetchPosters()
	h.prefetchBackdrops(720, 480)
	r, ok := a.nextWant()
	if !ok || r.thumb != "visible" {
		t.Fatal("preload displaced visible art")
	}
	posters, backdrops := 0, 0
	for {
		r, ok = a.nextWant()
		if !ok {
			break
		}
		if !r.prefetch {
			t.Fatal("preload must be background work")
		}
		if r.fade > 0 {
			backdrops++
			if r.w != 720 || r.h != 480 || r.bright != HeroBright || r.fade != HeroFade {
				t.Fatal("preloaded backdrop cannot reuse the visible cache entry")
			}
		} else {
			posters++
		}
	}
	if posters != 28 || backdrops != 5 {
		t.Fatalf("unbounded or missing preload: %d posters, %d backdrops", posters, backdrops)
	}
}

func TestHomeWarmsEveryRowAndColumn(t *testing.T) {
	a := NewArt(nil, 0, nil)
	h := &Home{app: &App{Art: a}, home: true}
	for row := 0; row < 10; row++ {
		hub := &plex.Hub{}
		for col := 0; col < 30; col++ {
			key := fmt.Sprintf("%d/%d", row, col)
			hub.Items = append(hub.Items, &plex.Item{Thumb: "poster/" + key, Art: "backdrop/" + key})
		}
		h.hubs = append(h.hubs, hub)
	}
	h.warmHomeArtwork(720, 480)
	h.warmHomeArtwork(720, 480)
	if len(a.warm) != 600 {
		t.Fatalf("want 600 unique requests, got %d", len(a.warm))
	}
	for i := 0; i < wantTTL+2; i++ {
		a.Frame()
	}
	a.Get("visible", PosterW, PosterH)
	a.Prefetch("nearby", PosterW, PosterH)
	for _, want := range []string{"visible", "nearby"} {
		r, ok := a.nextWant()
		if !ok || r.thumb != want {
			t.Fatalf("priority lost: got %s, want %s", r.thumb, want)
		}
	}
	count := 0
	for {
		_, ok := a.nextWant()
		if !ok {
			break
		}
		count++
	}
	if count != 600 {
		t.Fatalf("backlog expired or truncated: %d", count)
	}
	h.warmHomeArtwork(720, 480)
	if len(a.warm) != 0 {
		t.Fatal("completed warming restarted")
	}
}

func TestHomeWarmQueueRefreshesOnlyWhenRowsChange(t *testing.T) {
	a := NewArt(nil, 0, nil)
	h := &Home{app: &App{Art: a}, home: true}
	hub := &plex.Hub{Items: []*plex.Item{{Thumb: "first"}}}
	h.setHubs([]*plex.Hub{hub})
	h.warmHomeArtwork(720, 480)
	if !h.artworkWarmed {
		t.Fatal("full scan was not marked complete")
	}
	hub.Items = append(hub.Items, &plex.Item{Thumb: "new"})
	h.setHubs([]*plex.Hub{hub})
	h.warmHomeArtwork(720, 480)
	if len(a.warm) != 4 {
		t.Fatalf("new artwork did not join backlog: %d", len(a.warm))
	}
}

func TestHomeHoldDefersBackgroundUntilReleaseSettles(t *testing.T) {
	a := NewArt(nil, 0, nil)
	h := &Home{app: &App{Art: a}, home: true}
	h.setHubs([]*plex.Hub{homeRow("row", "a", "b", "c")})
	now := time.Now()
	h.Key(input.Event{Key: input.Right}, now)
	if !h.preloadAfter.After(now.Add(400 * time.Millisecond)) {
		t.Fatal("pause does not cover initial key repeat delay")
	}
	h.Key(input.Event{Key: input.Right, Repeat: true}, now.Add(time.Second))
	if !h.preloadAfter.After(now.Add(time.Second)) {
		t.Fatal("held navigation did not extend pause")
	}
	released := now.Add(2 * time.Second)
	h.Key(input.Event{Key: input.Right, Release: true}, released)
	if h.preloadAfter != released.Add(HeroRest) {
		t.Fatal("release did not allow final scroll to settle")
	}
}
