package ui

import "testing"

func TestArtVisibleBeforePrefetch(t *testing.T) {
	a := NewArt(nil, 0, nil)
	a.Get("visible", WallPW, WallPH)
	a.Prefetch("far", WallPW, WallPH)
	a.Prefetch("near", WallPW, WallPH)
	for _, want := range []string{"visible", "near", "far"} {
		r, ok := a.nextWant()
		if !ok || r.thumb != want {
			t.Fatalf("got %q, want %q", r.thumb, want)
		}
	}
}

func TestArtPromoteAndExpirePrefetch(t *testing.T) {
	a := NewArt(nil, 0, nil)
	a.Prefetch("old row", WallPW, WallPH)
	for i := 0; i <= wantTTL; i++ {
		a.Frame()
	}
	a.Prefetch("now visible", WallPW, WallPH)
	a.Get("now visible", WallPW, WallPH)
	a.Prefetch("now visible", WallPW, WallPH)
	a.Prefetch("new nearby row", WallPW, WallPH)
	r, ok := a.nextWant()
	if !ok || r.thumb != "now visible" || r.prefetch {
		t.Fatalf("visible promotion lost: %+v", r)
	}
	r, ok = a.nextWant()
	if !ok || r.thumb != "new nearby row" {
		t.Fatalf("new neighborhood missing: %+v", r)
	}
	if _, ok := a.nextWant(); ok {
		t.Fatal("stale neighborhood survived jump")
	}
}
