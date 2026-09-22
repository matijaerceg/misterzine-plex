package ui

import (
	"time"

	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/plex"
)

// Options is the settings screen: a short list of toggles.
type Options struct {
	app *App
	cur int
}

type option struct {
	label string
	get   func() bool // a toggle; nil otherwise
	set   func(bool)
	val   func() string // a stepped value; nil otherwise
	step  func(d int)   // d is -1 or +1
	do    func()        // an action
}

func (o *Options) items() []option {
	cfg := o.app.Cfg
	items := []option{
		{label: "Only show 4:3 media", get: func() bool { return cfg.FourThree }, set: func(v bool) { cfg.FourThree = v }},
		{label: "Autoplay next episode", get: func() bool { return !cfg.NoAutoplay }, set: func(v bool) { cfg.NoAutoplay = !v }},
		{label: "Video bitrate", val: func() string {
			value := mbps(cfg.BitrateKbps())
			if cfg.BitrateKbps() > DefaultBitrate {
				value += " (experimental)"
			}
			return value
		}, step: func(d int) {
			i := 0
			for j, b := range Bitrates {
				if b == cfg.BitrateKbps() {
					i = j
				}
			}
			i = max(0, min(len(Bitrates)-1, i+d))
			cfg.Bitrate = Bitrates[i]
		}},
		{label: "Video output", val: func() string {
			if o.app.videoLocked() {
				return "480i (CRT profile)"
			}
			if cfg.Progressive {
				return "480p (HDMI)"
			}
			return "480i (CRT)"
		}, do: o.app.chooseVideo},
		{label: "Theme music", get: func() bool { return !cfg.NoTheme }, set: func(v bool) { cfg.NoTheme = !v }},
		{label: "Navigation sounds", get: func() bool { return !cfg.NoTaps }, set: func(v bool) { cfg.NoTaps = !v }},
	}
	if cfg.Token != "" {
		items = append(items, option{label: "Choose server again", do: func() { o.app.Push(NewServerPicker(o.app)) }})
	}
	label := "Sign out"
	if cfg.Token != "" && cfg.AccountName != "" {
		label = "Sign out (" + cfg.AccountName + ")"
	}
	items = append(items, option{label: label, do: o.app.SignOut})
	return items
}

// mbps writes a kbit/s figure the way the other apps do: "4.5 Mbps".
func mbps(kbps int) string {
	whole, frac := kbps/1000, (kbps%1000)/100
	if frac == 0 {
		return itoa(whole) + " Mbps"
	}
	return itoa(whole) + "." + itoa(frac) + " Mbps"
}

// NewOptions makes the settings screen.
func NewOptions(app *App) *Options {
	app.loadAccountName()
	return &Options{app: app}
}

// Fetch optional display information off-thread, including for older saved logins.
// A failed lookup leaves sign-in intact and can retry when Options is reopened.
func (a *App) loadAccountName() {
	cfg := a.Cfg
	if cfg.Token == "" || cfg.AccountName != "" || a.accountLookup {
		return
	}
	id, token := cfg.ClientID, cfg.Token
	a.accountLookup = true
	go func() {
		name, err := plex.AccountName(id, token)
		a.Later(func() {
			a.accountLookup = false
			if a.Cfg != cfg || cfg.Token != token {
				return // signed out or changed account while the request was running
			}
			if err != nil || name == "" {
				return
			}
			next := *cfg
			next.AccountName = name
			if err := next.Save(); err != nil {
				a.Log.Printf("could not save account name")
				return
			}
			*cfg = next
		})
	}()
}

// Key handles one input event: up/down choose, OK/left/right toggle.
func (o *Options) Key(ev input.Event, now time.Time) {
	if ev.Release {
		return
	}
	items := o.items()
	switch ev.Key {
	case input.Up:
		if o.cur > 0 {
			o.cur--
		}
	case input.Down:
		if o.cur < len(items)-1 {
			o.cur++
		}
	case input.Enter, input.Left, input.Right:
		before := *o.app.Cfg
		it := items[o.cur]
		switch {
		case it.get != nil:
			it.set(!it.get())
		case it.step != nil:
			if ev.Key == input.Left {
				it.step(-1)
			} else {
				it.step(1)
			}
		default:
			if ev.Key == input.Enter {
				it.do()
			}
			return
		}
		if err := o.app.Cfg.Save(); err != nil {
			o.app.Log.Printf("config: %v", err)
			*o.app.Cfg = before
			o.app.Notice, o.app.NoticeAt = "Could not save settings. Check free space and retry.", now
			return
		}
		o.app.Reconfigured()
	}
}

// Draw paints the list of options with their values on the right.
func (o *Options) Draw(c *gfx.Canvas, now time.Time) bool {
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	f := o.app.F
	o.app.text(c, MenuX, SafeY, f.Title, gfx.Grey, "Options")
	y := ListY0
	for i, it := range o.items() {
		col := gfx.GreyHi
		if i == o.cur {
			col = gfx.White
			menuFocusBar(c, MenuX, y+9, f.Body.Height())
		}
		labelW := MenuWidth
		if it.get != nil {
			labelW -= f.Body.Width("Off") + 24
		} else if it.val != nil {
			labelW -= f.Body.Width(it.val()) + 24
		}
		o.app.text(c, MenuX, y+9, f.Body, col, f.Body.Fit(it.label, labelW))
		if it.get != nil {
			v := "Off"
			vc := gfx.GreyLo
			if it.get() {
				v, vc = "On", gfx.Amber
			}
			o.app.textRight(c, MenuRight, y+9, f.Body, vc, v)
		}
		if it.val != nil {
			o.app.textRight(c, MenuRight, y+9, f.Body, gfx.Amber, it.val())
		}
		y += MenuRowH
	}
	return false
}
