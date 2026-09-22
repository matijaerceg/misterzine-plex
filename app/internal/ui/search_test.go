package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
)

func TestSearchStaleAndFocus(t *testing.T) {
	s := &Search{app: &App{}, generation: 2, loading: true, key: 8}
	s.accept(1, []*plex.Item{{Title: "stale"}}, nil)
	if len(s.items) != 0 || !s.loading {
		t.Fatal("stale response applied")
	}
	s.accept(2, []*plex.Item{{Title: "current", Type: "movie"}}, nil)
	if s.key != 8 || s.results || s.loading {
		t.Fatal("response changed focus")
	}
	s.query = "C"
	s.Key(input.Event{Key: input.JumpFwd}, time.Now())
	if !s.results {
		t.Fatal("cannot enter results")
	}
	if !s.Back() || s.results || s.key != 8 || s.query != "" {
		t.Fatal("back did not clear query and restore keyboard")
	}
	if s.Back() || !s.closed {
		t.Fatal("back did not exit")
	}
	s.accept(2, []*plex.Item{{Title: "late"}}, nil)
	if len(s.items) != 0 {
		t.Fatal("closed search applied results")
	}
}

func TestSearchNavigationAndHistory(t *testing.T) {
	cfg := LoadConfig(filepath.Join(t.TempDir(), "config.json"))
	s := NewSearch(&App{Cfg: cfg})
	s.query = "SIM"
	s.remember()
	s.query = "BAT"
	s.remember()
	s.query = "sim"
	s.remember()
	if len(s.recent) != 2 || s.recent[0] != "sim" {
		t.Fatal("history not deduplicated")
	}
	if got := LoadConfig(cfg.path).RecentSearches; len(got) != 2 || got[0] != "sim" {
		t.Fatal("history not saved")
	}
	s.query = ""
	s.key = 5
	s.Key(input.Event{Key: input.Right}, time.Now())
	if !s.results {
		t.Fatal("d-pad cannot reach history")
	}
	s.Key(input.Event{Key: input.JumpBack}, time.Now())
	s.key = 29
	s.Key(input.Event{Key: input.Enter}, time.Now())
	if !s.numbers || s.keys()[0] != "0" {
		t.Fatal("number toggle failed")
	}
	s.key = 28
	s.Key(input.Event{Key: input.Enter}, time.Now())
	if s.query != "" {
		t.Fatal("clear failed")
	}
	cfg.SignOut()
	if len(cfg.RecentSearches) != 0 {
		t.Fatal("history survived signout")
	}
}

func TestSearchDebounce(t *testing.T) {
	queries := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries <- r.URL.Query().Get("query")
		fmt.Fprint(w, `<MediaContainer><Hub><Video type="movie" ratingKey="7" key="/library/metadata/7" title="Sim"/></Hub></MediaContainer>`)
	}))
	defer server.Close()
	a := &App{Plex: plex.New(server.URL, "", t.TempDir(), ""), later: make(chan func(), 32), Wake: make(chan struct{}, 1)}
	s := NewSearch(a)
	s.change("S")
	s.change("SI")
	s.change("SIM")
	select {
	case q := <-queries:
		if q != "SIM" {
			t.Fatalf("undebounced query: %s", q)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("search never started")
	}
	select {
	case f := <-a.later:
		f()
	case <-time.After(time.Second):
		t.Fatal("no completion")
	}
	if s.loading || len(s.items) != 1 {
		t.Fatal("completion not applied")
	}
	select {
	case <-queries:
		t.Fatal("extra request")
	default:
	}
	s.Back()
}

func TestSearchPhysicalKeyboardRouting(t *testing.T) {
	a := &App{later: make(chan func(), 32), Wake: make(chan struct{}, 1)}
	s := NewSearch(a)
	a.stack = []Screen{s}
	now := time.Now()
	// These legacy navigation aliases must type instead of moving or selecting.
	for _, ev := range []input.Event{
		{Keyboard: true, ScanCode: 0x1d, Text: 'w', Key: input.Up},
		{Keyboard: true, ScanCode: 0x1c, Text: 'a', Key: input.Left},
		{Keyboard: true, ScanCode: 0x1b, Text: 's', Key: input.Down},
		{Keyboard: true, ScanCode: 0x23, Text: 'd', Key: input.Right},
		{Keyboard: true, ScanCode: 0x29, Text: ' ', Key: input.Enter},
		{Keyboard: true, ScanCode: 0x16, Text: '!', Key: input.None},
	} {
		a.key(ev, now)
	}
	if s.query != "wasd !" || s.key != 0 || s.results {
		t.Fatal("typing moved focus", s.query, s.key)
	}
	a.key(input.Event{Keyboard: true, ScanCode: 0x66, Key: input.Back}, now)
	if s.query != "wasd " || len(a.stack) != 1 || s.closed {
		t.Fatal("Backspace navigated away")
	}
	a.key(input.Event{Keyboard: true, Text: 'x', Key: input.None, Release: true}, now)
	if s.query != "wasd " {
		t.Fatal("release inserted text")
	}
	// Finish the pending request without a real server and expose a result.
	s.cancel()
	s.loading = false
	s.items = []*plex.Item{{Title: "test", Type: "movie"}}
	a.key(input.Event{Keyboard: true, ScanCode: 0x5a, Key: input.Enter}, now)
	if !s.results || s.query != "wasd " {
		t.Fatal("Enter should focus results")
	}
	a.key(input.Event{Keyboard: true, ScanCode: 0x0d, Key: input.None}, now)
	if s.results {
		t.Fatal("Tab did not return to letters")
	}
	a.key(input.Event{Key: input.Right}, now)
	if s.key != 1 {
		t.Fatal("controller stopped working")
	}
	s.change("")
	a.key(input.Event{Keyboard: true, ScanCode: 0x66, Key: input.Back}, now)
	if s.closed {
		t.Fatal("Backspace on empty query exited")
	}
	a.key(input.Event{Keyboard: true, ScanCode: 0x76, Key: input.Back}, now)
	if !s.closed {
		t.Fatal("Escape did not exit")
	}
}

// Optional preview generation exercises the actual renderer with fictional data.
func TestSearchPreview(t *testing.T) {
	dir := os.Getenv("SEARCH_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set SEARCH_PREVIEW_DIR to render previews")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	a := New(nil, nil, nil, log.New(io.Discard, "", 0))
	s := NewSearch(a)
	s.query = "STAR"
	s.items = []*plex.Item{{Title: "Star Voyager", Type: "movie", Year: 1984}, {Title: "Starlight Stories", Type: "show", Year: 1992}, {Title: "The Last Star Pilot", Type: "movie", Year: 1987}}
	for _, name := range []string{"keyboard", "results", "recent"} {
		if name == "results" {
			s.results = true
			s.cur = 1
		}
		if name == "recent" {
			s.results = false
			s.query = ""
			s.recent = []string{"STAR", "SPACE", "BAT"}
		}
		c := gfx.NewCanvas(720, 480)
		for i := 0; i < 15; i++ {
			s.Draw(c, time.Now())
			time.Sleep(20 * time.Millisecond)
		}
		// 720 source pixels occupy a 640-wide 4:3 display.
		out := image.NewRGBA(image.Rect(0, 0, 640, 480))
		for y := 0; y < 480; y++ {
			for x := 0; x < 640; x++ {
				p := (y*720 + x*720/640) * 4
				out.SetRGBA(x, y, color.RGBA{c.Pix[p+2], c.Pix[p+1], c.Pix[p], 255})
			}
		}
		file, err := os.Create(filepath.Join(dir, "search-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, out)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
