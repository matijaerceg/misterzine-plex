package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"plexcrt/internal/plex"
)

// Aspects remembers the picture shape of shows, taken from each show's
// first episode, so the 4:3 filter can judge a show the way it judges a
// movie. A show is probed once and the answer kept on disk in the cache
// folder; until it is known the show stays visible.
type Aspects struct {
	mu    sync.Mutex
	known map[string]float64 // show rating key -> aspect (0: probed, unknown)
	path  string
}

const (
	aspectProbes = 8               // shows probed at once
	aspectBudget = 6 * time.Second // per batch; the rest stay visible until next time
)

func (a *Aspects) load(dir string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.path = filepath.Join(dir, "aspects.json")
	a.known = map[string]float64{}
	if data, err := os.ReadFile(a.path); err == nil {
		_ = json.Unmarshal(data, &a.known)
	}
}

func (a *Aspects) save() {
	a.mu.Lock()
	data, err := json.Marshal(a.known)
	path := a.path
	a.mu.Unlock()
	if err != nil || path == "" {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

// apply fills in the aspect of shows already known, without any request.
func (a *Aspects) apply(items []*plex.Item) (unknown []*plex.Item) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, it := range items {
		if it.Type != "show" || it.Aspect != 0 {
			continue
		}
		if v, ok := a.known[it.RatingKey]; ok {
			it.Aspect = v
		} else {
			unknown = append(unknown, it)
		}
	}
	return unknown
}

// measure fills in the aspect of every show in items, probing the ones not
// seen before (a few at a time, within a time budget). Safe from any
// goroutine; meant for the workers that fetch listings.
func (a *App) measureShows(items []*plex.Item) {
	if a.Plex == nil || a.Cfg == nil || !a.Cfg.FourThree {
		return
	}
	unknown := a.aspects.apply(items)
	if len(unknown) == 0 {
		return
	}
	client := a.Plex
	deadline := time.Now().Add(aspectBudget)
	sem := make(chan struct{}, aspectProbes)
	var wg sync.WaitGroup
	for _, it := range unknown {
		if time.Now().After(deadline) {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(it *plex.Item) {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := client.ShowAspect(it.RatingKey)
			if err != nil {
				return // left unknown: visible now, probed again next time
			}
			a.aspects.mu.Lock()
			a.aspects.known[it.RatingKey] = v
			a.aspects.mu.Unlock()
			it.Aspect = v
		}(it)
	}
	wg.Wait()
	a.aspects.save()
}

// measureHubs runs measureShows over every row of a home fetch.
func (a *App) measureHubs(hubs []*plex.Hub) {
	var items []*plex.Item
	for _, hub := range hubs {
		items = append(items, hub.Items...)
	}
	a.measureShows(items)
}
