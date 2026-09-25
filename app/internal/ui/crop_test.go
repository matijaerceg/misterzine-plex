package ui

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"plexcrt/internal/input"
	"plexcrt/internal/plex"
)

// The presenter's own test (tools/plexfb_scale_test.c) reads the same table
// for the exact part kept; here the app must agree on whether anything is cut.
func TestCropCutsMatchesPresenterCases(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "tools", "testdata", "crop_cases.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) != 13 {
			continue
		}
		v := map[int]int{}
		for _, i := range []int{0, 1, 2, 3, 4, 6, 7, 11, 12} {
			x, err := strconv.Atoi(fields[i])
			if err != nil {
				t.Fatalf("bad case %q", sc.Text())
			}
			v[i] = x
		}
		a, b, _ := strings.Cut(fields[8], "/")
		num, _ := strconv.ParseFloat(a, 64)
		den, _ := strconv.ParseFloat(b, 64)
		c := Crop(fields[5])
		if c.valid() != c {
			t.Fatalf("%q: not a crop", sc.Text())
		}
		g := Geometry{Left: v[0], Top: v[1], Right: v[2], Bottom: v[3], Width: v[4]}
		want := v[11] != v[6] || v[12] != v[7]
		if got := g.Cuts(c, num/den); got != want {
			t.Errorf("%q: Cuts says %v", sc.Text(), got)
		}
		n++
	}
	if n < 20 {
		t.Fatalf("only %d cases read", n)
	}
}

func TestCropNamesAndLabels(t *testing.T) {
	for _, tc := range []struct {
		c     Crop
		valid Crop
		label string
	}{{"", CropOff, "Off"}, {"off", CropOff, "Off"}, {"14:9", Crop14x9, "14:9"}, {"fill", CropFill, "Fill"}, {"4:3", CropOff, "Off"}} {
		if tc.c.valid() != tc.valid || tc.c.Label() != tc.label {
			t.Errorf("%q: %q %q", tc.c, tc.c.valid(), tc.c.Label())
		}
	}
}

func cropApp(t *testing.T) *App {
	t.Helper()
	a := New(nil, nil, &Player{CropFile: filepath.Join(t.TempDir(), "plexfb.crop")}, log.New(io.Discard, "", 0))
	a.Cfg = LoadConfig(filepath.Join(t.TempDir(), "settings.json"))
	return a
}

func cropFile(t *testing.T, a *App) string {
	t.Helper()
	b, err := os.ReadFile(a.Player.CropFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOptionsCropStepsAndSaves(t *testing.T) {
	a := cropApp(t)
	o := NewOptions(a)
	row := -1
	for i, it := range o.items() {
		if it.label == "Video crop" {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no Video crop row")
	}
	o.cur = row
	var seen []string
	for _, k := range []input.Key{input.Right, input.Right, input.Enter, input.Left} {
		o.Key(input.Event{Key: k}, time.Now())
		seen = append(seen, o.items()[row].val())
	}
	if strings.Join(seen, ",") != "14:9,Fill,Fill,14:9" {
		t.Fatalf("stepped through %v", seen)
	}
	if saved := LoadConfig(a.Cfg.path); saved.Crop != Crop14x9 {
		t.Fatalf("saved %q", saved.Crop)
	}
}

func playingCrop(a *App, aspect float64) *Playing {
	it := &plex.Item{Title: "Film", Aspect: aspect}
	p := &Playing{app: a, item: it, send: func(string) {}, visible: true}
	for i, b := range osdButtons {
		if b == "More" {
			p.focus = i
		}
	}
	return p
}

func pressPlay(p *Playing, keys ...input.Key) {
	for _, k := range keys {
		p.Key(input.Event{Key: k}, time.Now())
	}
}

func TestPlaybackMenuCropsThisPlaybackOnly(t *testing.T) {
	a := cropApp(t)
	a.Cfg.Crop = Crop14x9
	a.crop = a.Cfg.Crop.valid() // as PlayQueue starts a playback
	p := playingCrop(a, 1.78)
	pressPlay(p, input.Enter)
	if p.list == nil || p.list.kind != "More" || p.list.vals[0] != "14:9" {
		t.Fatalf("More did not open on the crop in force: %+v", p.list)
	}
	pressPlay(p, input.Enter)
	if p.list == nil || p.list.kind != "Crop" || p.list.set != 1 || p.list.cur != 1 {
		t.Fatalf("the crop choices did not open on 14:9: %+v", p.list)
	}
	pressPlay(p, input.Down, input.Enter)
	if p.list != nil || a.crop != CropFill || cropFile(t, a) != "fill\n" {
		t.Fatalf("choosing Fill: list %+v, crop %q, file %q", p.list, a.crop, cropFile(t, a))
	}
	if a.Cfg.Crop != Crop14x9 || LoadConfig(a.Cfg.path).Crop != "" {
		t.Fatalf("the playback menu changed the saved crop: %q", a.Cfg.Crop)
	}
	// back a level at a time: the choices, More, the controls
	pressPlay(p, input.Enter, input.Enter)
	if p.list == nil || p.list.kind != "Crop" || p.list.set != 2 {
		t.Fatalf("reopened choices: %+v", p.list)
	}
	pressPlay(p, input.Back)
	if p.list == nil || p.list.kind != "More" || p.list.vals[0] != "Fill" {
		t.Fatalf("Back from the choices: %+v", p.list)
	}
	pressPlay(p, input.Left)
	if p.list != nil || !p.visible {
		t.Fatalf("Left from More should leave the controls up: list %+v", p.list)
	}
}

func TestMoreIsDimmedWhenNoCropWouldCut(t *testing.T) {
	a := cropApp(t)
	more := len(osdButtons) - 1
	for _, tc := range []struct {
		aspect float64
		dimmed bool
	}{{1.33, true}, {1.78, false}, {2.39, false}, {1.37, false}, {0.5625, false}, {0, false}} {
		if got := playingCrop(a, tc.aspect).dimmed(more); got != tc.dimmed {
			t.Errorf("aspect %v: dimmed %v", tc.aspect, got)
		}
	}
	// a dimmed More does not open
	p := playingCrop(a, 1.33)
	pressPlay(p, input.Enter)
	if p.list != nil {
		t.Fatal("More opened for a 4:3 picture")
	}
	// on a narrow picture area fill cuts even 4:3
	a.Cfg.Geometry = Geometry{Left: 120, Right: 120}
	if playingCrop(a, 1.33).dimmed(more) {
		t.Fatal("More dimmed where fill would cut the sides of 4:3")
	}
}

func TestPlayerSetCropReplacesTheFile(t *testing.T) {
	a := cropApp(t)
	for _, tc := range []struct{ c, want Crop }{{CropFill, "fill"}, {"bogus", "off"}, {Crop14x9, "14:9"}} {
		if err := a.Player.SetCrop(tc.c); err != nil {
			t.Fatal(err)
		}
		if got := cropFile(t, a); got != string(tc.want)+"\n" {
			t.Fatalf("%q written as %q", tc.c, got)
		}
	}
	if _, err := os.Stat(a.Player.CropFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("the temporary file was left behind")
	}
	if err := (&Player{}).SetCrop(CropFill); err != nil {
		t.Fatal("no crop file should be nothing to do")
	}
}
