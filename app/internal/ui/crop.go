package ui

import "os"

// Crop is how much of a picture the presenter cuts away (arm/plexfb.c
// crop_rect): nothing, the sides of anything wider than 14:9, or whatever
// overhangs the picture area, so the picture fills it (the sides of a wide
// picture on a 4:3 area, the top and bottom of a narrow one). Options holds
// the one each playback starts with; the playback menu changes it until the
// playback ends.
type Crop string

const (
	CropOff  Crop = "off"
	Crop14x9 Crop = "14:9"
	CropFill Crop = "fill"
)

// Crops are the choices, in the order they are offered.
var Crops = []struct {
	Mode  Crop
	Label string
}{{CropOff, "Off"}, {Crop14x9, "14:9"}, {CropFill, "Fill"}}

// index is c's place in Crops; anything else (nothing saved) is Off.
func (c Crop) index() int {
	for i, x := range Crops {
		if x.Mode == c {
			return i
		}
	}
	return 0
}

// Label is what Options and the playback menu call c.
func (c Crop) Label() string { return Crops[c.index()].Label }

// valid is c, or Off for anything that is not a crop.
func (c Crop) valid() Crop { return Crops[c.index()].Mode }

// Cuts reports whether crop c cuts anything from a picture of display
// aspect `aspect` in the picture area. It must decide as crop_rect in
// arm/plexfb.c does (both are tested against tools/testdata/crop_cases.txt),
// so the sums use variables in the same order.
func (g Geometry) Cuts(c Crop, aspect float64) bool {
	g = g.Normal()
	target := 14.0 / 9
	switch c.valid() {
	case Crop14x9:
		return aspect > target*1.01
	case CropFill:
		eightNinths, k := 8.0/9, float64(g.Width)/1000
		target = eightNinths * float64(720-g.Left-g.Right) / (float64(480-g.Top-g.Bottom) * k)
		return aspect > target*1.01 || aspect < target/1.01
	}
	return false
}

// SetCrop tells the presenter which crop to use: it reads CropFile when it
// starts and looks again every tenth of a second. The file is replaced
// whole, so the presenter never reads half of it.
func (p *Player) SetCrop(c Crop) error {
	if p.CropFile == "" {
		return nil
	}
	tmp := p.CropFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(string(c.valid())+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.CropFile)
}

// VideoCrop is the crop of the playback under way, read at each start.
func (a *App) VideoCrop() Crop { return a.crop }

// setCrop changes the crop until this playback ends.
func (a *App) setCrop(c Crop) {
	a.crop = c.valid()
	if a.Player == nil {
		return
	}
	if err := a.Player.SetCrop(a.crop); err != nil {
		a.Log.Printf("crop: %v", err)
	}
}
