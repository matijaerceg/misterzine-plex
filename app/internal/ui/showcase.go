package ui

import (
	"plexcrt/internal/plex"
	"strings"
	"time"
)

// libraryLabel changes presentation only; Plex keys and stored names stay intact.
func (a *App) libraryLabel(s plex.Section) string {
	if !a.Showcase {
		return s.Title
	}
	label := "Library"
	if s.Type == "movie" {
		label = "Movies"
	}
	if s.Type == "show" {
		label = "TV Shows"
	}
	count := 0
	for _, other := range a.secs {
		if other.Type == s.Type {
			count++
		}
		if other.Key == s.Key {
			break
		}
	}
	if count > 1 {
		label += " " + itoa(count)
	}
	return label
}

func (a *App) hubLabel(h *plex.Hub) string {
	if !a.Showcase {
		return h.Title
	}
	// Library rows carry a See all tile. Other rows retain their titles.
	for _, it := range h.Items {
		if it.Type != "more" {
			continue
		}
		prefix := "Recently Added"
		if strings.Contains(h.Ident, "recentlyaired") {
			prefix = "Recently Aired"
		}
		if s, ok := a.section(it.Key); ok {
			return prefix + " in " + a.libraryLabel(s)
		}
		return prefix
	}
	return h.Title
}

func (a *App) toggleShowcase(o *Options, now time.Time) {
	a.Showcase = !a.Showcase
	// Drop saved drawer/underlying screen snapshots so Back cannot reveal old labels.
	if len(a.stack) > 0 {
		if h, ok := a.stack[0].(*Home); ok {
			h.fade.Done()
			h.page, h.pagePrev = nil, nil
			h.pageKey = ""
			a.stack = []Screen{h, o}
		}
	}
	a.Notice = "Showcase mode off"
	if a.Showcase {
		a.Notice = "Showcase mode on: library names and totals hidden"
	}
	a.NoticeAt = now
	a.dirty = true
}
