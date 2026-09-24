package ui

import (
	"errors"
	"io"
	"log"
	"plexcrt/internal/plex"
	"testing"
	"time"
)

func homeRow(id string, keys ...string) *plex.Hub {
	h := &plex.Hub{Ident: id}
	for _, key := range keys {
		h.Items = append(h.Items, &plex.Item{RatingKey: key, Key: key})
	}
	return h
}

func TestHomeRefreshPreservesSelection(t *testing.T) {
	h := &Home{app: &App{}}
	h.applyHome([]*plex.Hub{homeRow("continue", "a"), homeRow("recent", "b", "c", "d", "e", "f", "g")})
	h.row, h.col[1] = 1, 5
	h.colX[1].Set(2 * PosterPitch)
	h.applyHome([]*plex.Hub{homeRow("recent", "x", "b", "c", "d", "e", "f", "g")})
	if h.row != 0 || h.Focused().RatingKey != "g" || h.colX[0].Target() != 3*PosterPitch {
		t.Fatal("refresh lost the selected title or scroll position")
	}
	h.applyHome([]*plex.Hub{homeRow("recent", "x")})
	if h.Focused().RatingKey != "x" || h.colX[0].Target() != 0 {
		t.Fatal("removed selection was not clamped into view")
	}
	h.applyHome(nil)
	if !h.empty || h.Focused() != nil {
		t.Fatal("empty refresh retained a selection")
	}
}

func TestHomeRefreshRetainsRowsOnFailureAndRecovers(t *testing.T) {
	h := &Home{app: &App{Log: log.New(io.Discard, "", 0)}, home: true}
	h.applyHome([]*plex.Hub{homeRow("continue", "a")})
	now := time.Now()
	h.refreshResult = make(chan homeResult, 1)
	h.refreshResult <- homeResult{err: errors.New("offline")}
	h.pollHome(now)
	if h.Focused().RatingKey != "a" || h.err != nil || h.refreshResult != nil {
		t.Fatal("failed refresh discarded usable rows")
	}
	if !h.refreshAt.Equal(now.Add(30 * time.Second)) {
		t.Fatal("failed refresh did not back off")
	}
	h.refreshResult = make(chan homeResult, 1)
	h.refreshResult <- homeResult{hubs: []*plex.Hub{homeRow("continue", "b")}}
	h.pollHome(now)
	if h.Focused().RatingKey != "b" || !h.app.dirty {
		t.Fatal("successful refresh was not applied")
	}
}

func TestFilterChangeRefetchesHomeOffThread(t *testing.T) {
	app := &App{Log: log.New(io.Discard, "", 0), Cfg: &Config{}}
	h := &Home{app: app, home: true}
	h.applyHome([]*plex.Hub{homeRow("continue", "a")})
	h.refreshAt = time.Now().Add(time.Hour)
	app.stack = []Screen{h}
	app.Reconfigured() // must return at once: no fetch on this thread
	if !app.homeUpdating() || !h.refreshAt.IsZero() {
		t.Fatal("filter change did not schedule an immediate background refresh")
	}
	if h.Focused().RatingKey != "a" {
		t.Fatal("rows were dropped before the new ones arrived")
	}
	h.refreshResult = make(chan homeResult, 1)
	h.refreshResult <- homeResult{hubs: []*plex.Hub{homeRow("continue", "b")}}
	h.pollHome(time.Now())
	if app.homeUpdating() || h.Focused().RatingKey != "b" || !app.dirty {
		t.Fatal("refresh result did not clear the updating state")
	}
}

func TestHomeInitialEmptyAndUnchangedRefresh(t *testing.T) {
	h := &Home{app: &App{}}
	h.applyHome(nil)
	if !h.empty {
		t.Fatal("initial empty result must show the empty state")
	}
	h.applyHome([]*plex.Hub{homeRow("continue", "a")})
	h.pageKey = "cached"
	h.applyHome([]*plex.Hub{homeRow("continue", "a")})
	if h.pageKey != "cached" {
		t.Fatal("unchanged refresh invalidated the composed page")
	}
}
