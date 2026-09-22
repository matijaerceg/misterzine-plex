package ui

import (
	"time"

	"plexcrt/internal/plex"
)

// A single short-lived listing bounds speculative work and memory. All fields
// belong to the render thread; the fetch worker only hands back its result.
type seasonData struct {
	client  *plex.Client
	key     string
	eps     []*plex.Item
	err     error
	loading bool
	until   time.Time
	wait    []func()
}

func (a *App) seasonEpisodes(sea *plex.Item, retry bool) *seasonData {
	d := a.seasonData
	if d != nil && d.client == a.Plex && d.key == sea.Key &&
		(d.loading || (time.Now().Before(d.until) && (!retry || d.err == nil))) {
		return d
	}
	d = &seasonData{client: a.Plex, key: sea.Key, loading: true}
	a.seasonData = d
	go func() {
		eps, err := d.client.Items(d.key, nil, 500)
		a.Later(func() {
			d.eps, d.err, d.loading = eps, err, false
			d.until = time.Now().Add(30 * time.Second)
			for _, apply := range d.wait {
				apply()
			}
			d.wait = nil
		})
	}()
	return d
}

func (a *App) prefetchSeason(sea, show *plex.Item) {
	d := a.seasonEpisodes(sea, false)
	// The season uses a smaller logo than the show; prepare that variant too.
	a.Art.get(artReq{thumb: show.Logo, w: SeasonLogoW, h: SeasonLogoH, logo: true, prefetch: true})
	if d.loading || d.err != nil {
		return
	}
	cur := 0
	for i, ep := range d.eps {
		if ep.ViewCount == 0 {
			cur = i
			break
		}
	}
	first := max(0, cur-StillsAcross+1)
	for i := min(len(d.eps), first+StillsAcross) - 1; i >= first; i-- {
		ep := d.eps[i]
		thumb := ep.Still
		if thumb == "" {
			thumb = ep.Thumb
		}
		a.Art.Prefetch(thumb, StillW, StillH)
	}
}
