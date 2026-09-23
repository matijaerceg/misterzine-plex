package ui

import (
	"net/url"
	"strings"
	"sync"

	"plexcrt/internal/plex"
)

// Pager is a sparse view of a long server listing: items arrive in pages of
// pageSize around whatever the cursor is near, fetched in the background, so
// a 6,000-title library opens instantly and the alphabet jump lands anywhere.
type Pager struct {
	client *plex.Client
	path   string
	query  url.Values
	mu     sync.Mutex
	items  map[int]*plex.Item
	total  int
	loaded bool
	err    error
	inFlt  map[int]bool // page index being fetched
	Wake   chan struct{}
	// with a filter the listing is walked page by page from the start and
	// the kept items packed into list; total is then what has been kept
	// so far, and the alphabet index does not apply
	filter func(*plex.Item) bool
	// Prepare, if set, completes a page's items (in the fetching goroutine)
	// before the filter sees them: shows are probed for their shape
	Prepare func([]*plex.Item)
	list    []*plex.Item
	next    int  // next raw page to fetch
	done    bool // every raw page has been seen
}

const pageSize = 60

// NewPager starts loading the first page; wake is signalled as pages land.
// A filter, if given, hides items it rejects; prepare, if given, completes
// each page's items before the filter sees them.
func NewPager(c *plex.Client, path string, q url.Values, wake chan struct{}, filter func(*plex.Item) bool, prepare func([]*plex.Item)) *Pager {
	p := &Pager{client: c, path: path, query: q, items: map[int]*plex.Item{}, inFlt: map[int]bool{},
		Wake: wake, total: -1, filter: filter, Prepare: prepare}
	p.Want(0)
	return p
}

// Done reports whether a filtered walk has seen the whole listing.
func (p *Pager) Done() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.filter == nil || p.done
}

// Filtered reports whether the listing is a filtered walk (no letter index).
func (p *Pager) Filtered() bool { return p.filter != nil }

// Total is the listing size, or -1 until the first page lands.
func (p *Pager) Total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.filter != nil {
		if !p.loaded {
			return -1
		}
		return len(p.list)
	}
	return p.total
}

// Err is the last fetch error.
func (p *Pager) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// Get returns item i if loaded; nil queues its page.
func (p *Pager) Get(i int) *plex.Item {
	p.mu.Lock()
	var it *plex.Item
	if p.filter != nil {
		if i >= 0 && i < len(p.list) {
			it = p.list[i]
		}
	} else {
		it = p.items[i]
	}
	p.mu.Unlock()
	if it == nil {
		p.Want(i)
	}
	return it
}

// Want makes sure the page holding i (and its neighbours) is loading.
func (p *Pager) Want(i int) {
	if i < 0 {
		return
	}
	if p.filter != nil {
		// keep a page of kept items ahead of the cursor, one fetch at a time
		p.mu.Lock()
		fetch := !p.done && len(p.inFlt) == 0 && i+pageSize/2 >= len(p.list)
		if fetch {
			p.inFlt[p.next] = true
		}
		n := p.next
		p.mu.Unlock()
		if fetch {
			go p.fetch(n)
		}
		return
	}
	pg := i / pageSize
	for _, n := range []int{pg, pg + 1} {
		if n < 0 {
			continue
		}
		p.mu.Lock()
		if p.total >= 0 && n*pageSize >= p.total {
			p.mu.Unlock()
			continue
		}
		if p.inFlt[n] || p.items[n*pageSize] != nil {
			p.mu.Unlock()
			continue
		}
		p.inFlt[n] = true
		p.mu.Unlock()
		go p.fetch(n)
	}
}

func (p *Pager) fetch(n int) {
	q := url.Values{}
	for k, v := range p.query {
		q[k] = v
	}
	// the wall needs titles, art and progress only; the rest is fetched per item
	q.Set("excludeFields", "summary,tagline")
	ex := "Genre,Director,Writer,Role,Country,Producer,Media,Guid,Collection,Label,Field,Image"
	if p.filter != nil {
		ex = strings.Replace(ex, "Media,", "", 1) // the filter reads the media's aspect
	}
	q.Set("excludeElements", ex)
	items, total, err := p.client.Page(p.path, q, n*pageSize, pageSize)
	if err == nil && p.filter != nil && p.Prepare != nil {
		p.Prepare(items)
	}
	p.mu.Lock()
	delete(p.inFlt, n)
	if err != nil {
		p.err = err
	} else {
		p.err = nil
		p.total = total
		p.loaded = true
		if p.filter != nil {
			for _, it := range items {
				if p.filter(it) {
					p.list = append(p.list, it)
				}
			}
			p.next = n + 1
			if n*pageSize+len(items) >= total || len(items) == 0 {
				p.done = true
			}
			if !p.done {
				// keep walking to the end, so the count and the scrollbar
				// settle within seconds and the letter index can be built
				p.inFlt[p.next] = true
				go p.fetch(p.next)
			}
		} else {
			for i, it := range items {
				p.items[n*pageSize+i] = it
			}
		}
	}
	p.mu.Unlock()
	select {
	case p.Wake <- struct{}{}:
	default:
	}
}

// Letters builds a first-letter index from a finished filtered walk.
func (p *Pager) Letters() []plex.Letter {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []plex.Letter
	for _, it := range p.list {
		k := "#"
		if title := it.SortLabel(); len(title) > 0 {
			ch := title[0]
			if ch >= 'a' && ch <= 'z' {
				ch -= 'a' - 'A'
			}
			if ch >= 'A' && ch <= 'Z' {
				k = string(ch)
			}
		}
		if len(out) > 0 && out[len(out)-1].Key == k {
			out[len(out)-1].Size++
		} else {
			out = append(out, plex.Letter{Key: k, Size: 1})
		}
	}
	return out
}

// Replace swaps one item (after playback refreshed it).
func (p *Pager) Replace(i int, it *plex.Item) {
	p.mu.Lock()
	if p.filter != nil {
		if i >= 0 && i < len(p.list) {
			p.list[i] = it
		}
	} else {
		p.items[i] = it
	}
	p.mu.Unlock()
}
