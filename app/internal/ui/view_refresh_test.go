package ui

import (
	"errors"
	"plexcrt/internal/plex"
	"testing"
	"time"
)

func TestPeriodicRefreshLifecycle(t *testing.T) {
	now := time.Now()
	a := &App{Wake: make(chan struct{}, 1)}
	r := &viewRefresh{}
	calls := make(chan struct{}, 2)
	release := make(chan struct{})
	fetch := func() ([]*plex.Item, error) {
		calls <- struct{}{}
		<-release
		return []*plex.Item{{RatingKey: "fresh"}}, nil
	}
	applied := 0
	apply := func(items []*plex.Item) { applied++ }
	r.poll(a, now, fetch, apply)
	if r.pending != nil {
		t.Fatal("should wait before first refresh")
	}
	r.poll(a, now.Add(viewRefreshInterval), fetch, apply)
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	pending := r.pending
	r.poll(a, now.Add(2*viewRefreshInterval), fetch, apply)
	if r.pending != pending {
		t.Fatal("overlapping fetch")
	}
	// A local edit or season change invalidates an older request.
	r.reset()
	close(release)
	select {
	case <-a.Wake:
	case <-time.After(time.Second):
		t.Fatal("worker did not finish")
	}
	r.poll(a, now, fetch, apply)
	if applied != 0 {
		t.Fatal("applied stale result after reset")
	}
	r.pending = make(chan viewResult, 1)
	r.pending <- viewResult{err: errors.New("offline")}
	r.poll(a, now, fetch, apply)
	if applied != 0 || !r.next.Equal(now.Add(viewRefreshInterval)) {
		t.Fatal("failure should preserve data and back off")
	}
	r.pending = make(chan viewResult, 1)
	r.pending <- viewResult{items: []*plex.Item{{RatingKey: "fresh"}}}
	r.poll(a, now, fetch, apply)
	if applied != 1 || !a.dirty {
		t.Fatal("successful result not presented")
	}
}

func TestWatchRefreshPreservesListAndDetails(t *testing.T) {
	a := &plex.Item{RatingKey: "a", Title: "Original", PartID: "streams", ViewOffset: 10}
	b := &plex.Item{RatingKey: "b", ViewCount: 1}
	items := []*plex.Item{a, b}
	updateWatchState(items, []*plex.Item{{RatingKey: "new"}, {RatingKey: "a", ViewOffset: 40, ViewCount: 2, Viewed: 3}})
	if len(items) != 2 || items[0] != a || items[1] != b || a.Title != "Original" || a.PartID != "streams" || b.ViewCount != 1 {
		t.Fatal("refresh changed membership, order, or details")
	}
	if a.ViewOffset != 40 || a.ViewCount != 2 || a.Viewed != 3 {
		t.Fatal("watch state not updated")
	}
}

func TestPreplayRefreshPreservesAction(t *testing.T) {
	p := &Preplay{app: &App{}, item: &plex.Item{RatingKey: "movie"}}
	p.rebuild()
	p.cur = 1 // Mark watched, before Resume inserts an extra action.
	p.periodic.pending = make(chan viewResult, 1)
	p.periodic.pending <- viewResult{items: []*plex.Item{{RatingKey: "movie", ViewOffset: 90, ViewCount: 1}}}
	p.pollRefresh(time.Now())
	if p.actions[p.cur] != "Mark unwatched" || p.item.ViewOffset != 90 {
		t.Fatal("refresh moved focus away from the mark action")
	}
	p.cur = -1
	p.periodic.pending = make(chan viewResult, 1)
	p.periodic.pending <- viewResult{items: []*plex.Item{{RatingKey: "movie"}}}
	p.pollRefresh(time.Now())
	if p.cur != -1 {
		t.Fatal("refresh lost synopsis focus")
	}
}
