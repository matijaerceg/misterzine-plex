package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"plexcrt/internal/plex"
)

// A show is judged by its first episode, probed once and remembered on disk;
// an unprobed show stays visible.
func TestShowsAreMeasuredOnceForTheFilter(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !strings.HasSuffix(r.URL.Path, "/allLeaves") || r.URL.Query().Get("X-Plex-Container-Size") != "1" {
			t.Errorf("unexpected probe %s", r.URL)
		}
		aspect := "1.78"
		if strings.Contains(r.URL.Path, "/metadata/43/") {
			aspect = "1.33"
		}
		fmt.Fprintf(w, `<MediaContainer><Video ratingKey="9" type="episode" title="Pilot"><Media aspectRatio="%s"/></Video></MediaContainer>`, aspect)
	}))
	defer server.Close()
	dir := t.TempDir()
	a := &App{Plex: plex.New(server.URL, "", dir, "test"), Cfg: &Config{FourThree: true}}
	a.SetCacheDir(dir)
	wide := &plex.Item{RatingKey: "16", Type: "show", Title: "Wide"}
	square := &plex.Item{RatingKey: "43", Type: "show", Title: "Square"}
	movie := &plex.Item{RatingKey: "7", Type: "movie", Aspect: 2.35}
	items := []*plex.Item{wide, square, movie}
	if !a.Keep(wide) || !a.Keep(square) {
		t.Fatal("an unmeasured show must stay visible")
	}
	a.measureShows(items)
	if requests.Load() != 2 {
		t.Fatalf("probed %d shows, want 2 (movies carry their own aspect)", requests.Load())
	}
	if a.Keep(wide) || !a.Keep(square) || a.Keep(movie) {
		t.Fatal("filter did not use the measured shapes")
	}

	// a fresh app on the same cache folder knows the answers without asking
	b := &App{Plex: plex.New(server.URL, "", dir, "test"), Cfg: &Config{FourThree: true}}
	b.SetCacheDir(dir)
	again := []*plex.Item{{RatingKey: "16", Type: "show"}, {RatingKey: "43", Type: "show"}}
	b.measureShows(again)
	if requests.Load() != 2 {
		t.Fatal("remembered shows were probed again")
	}
	if b.Keep(again[0]) || !b.Keep(again[1]) {
		t.Fatal("remembered shapes were not applied")
	}

	// with the filter off nothing is probed
	c := &App{Plex: plex.New(server.URL, "", t.TempDir(), "test"), Cfg: &Config{}}
	c.SetCacheDir(t.TempDir())
	c.measureShows([]*plex.Item{{RatingKey: "99", Type: "show"}})
	if requests.Load() != 2 {
		t.Fatal("probed with the filter off")
	}
}
