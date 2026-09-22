package ui

import (
	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
	"testing"
	"time"
)

func TestShowcaseLabels(t *testing.T) {
	sections := []plex.Section{{Key: "1", Title: "Private movies", Type: "movie"}, {Key: "2", Title: "Private shows", Type: "show"}, {Key: "3", Title: "Other movies", Type: "movie"}}
	a := &App{Showcase: true, secs: sections}
	for i, want := range []string{"Movies", "TV Shows", "Movies 2"} {
		if got := a.libraryLabel(sections[i]); got != want {
			t.Fatalf("label %d: %q", i, got)
		}
	}
	h := &plex.Hub{Title: "Recently Added in Private shows", Items: []*plex.Item{{Type: "show", Title: "Example Show", ViewCount: 1}, {Type: "more", Key: "2"}}}
	if got := a.hubLabel(h); got != "Recently Added in TV Shows" {
		t.Fatal(got)
	}
	if h.Items[0].Title != "Example Show" || h.Items[0].ViewCount != 1 || h.Title != "Recently Added in Private shows" {
		t.Fatal("source data changed")
	}
	a.Showcase = false
	if a.libraryLabel(sections[0]) != sections[0].Title || a.hubLabel(h) != h.Title {
		t.Fatal("normal labels not restored")
	}
}

func TestShowcaseActivationClearsSnapshots(t *testing.T) {
	a := &App{Cfg: &Config{}, Version: "1.2.3"}
	h := &Home{app: a, page: gfx.NewCanvas(2, 2), pagePrev: gfx.NewCanvas(2, 2), pageKey: "old"}
	o := &Options{app: a}
	a.stack = []Screen{h, &Drawer{app: a}, o}
	o.cur = len(o.items()) - 1
	now := time.Now()
	for i := 0; i < 2; i++ {
		o.Key(input.Event{Key: input.Enter}, now)
	}
	o.Key(input.Event{Key: input.Enter, Repeat: true}, now)
	if a.Showcase {
		t.Fatal("repeat activated toggle")
	}
	o.Key(input.Event{Key: input.Enter}, now)
	if !a.Showcase || len(a.stack) != 2 || a.stack[1] != o || h.page != nil || h.pagePrev != nil || h.pageKey != "" {
		t.Fatal("privacy snapshots retained")
	}
	for i := 0; i < 3; i++ {
		o.Key(input.Event{Key: input.Enter}, now)
	}
	if a.Showcase {
		t.Fatal("toggle did not turn off")
	}
}

func TestShowcaseHidesNumericMarker(t *testing.T) {
	w := &Wall{app: &App{Showcase: true}, view: 1, cur: 12}
	if got := w.marker(900); got != "" {
		t.Fatal(got)
	}
	w.app.Showcase = false
	w.pager = &Pager{}
	if got := w.marker(900); got != "13 / 900" {
		t.Fatal(got)
	}
}
