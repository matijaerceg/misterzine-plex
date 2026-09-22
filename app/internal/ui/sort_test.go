package ui

import (
	"plexcrt/internal/plex"
	"testing"
)

func TestScrollbarUsesSortTitle(t *testing.T) {
	items := []*plex.Item{{Title: "The 'Burbs", SortTitle: "'Burbs"}, {Title: "The Matrix", SortTitle: "Matrix"}, {Title: "Zulu"}}
	p := &Pager{filter: func(*plex.Item) bool { return true }, list: items, done: true}
	w := &Wall{pager: p}
	for i, want := range []string{"'", "M", "Z"} {
		w.cur = i
		if got := w.marker(3); got != want {
			t.Fatalf("marker %d: got %q want %q", i, got, want)
		}
	}
	letters := p.Letters()
	for i, want := range []string{"#", "M", "Z"} {
		if letters[i].Key != want {
			t.Fatalf("index %d: %q", i, letters[i].Key)
		}
	}
}
