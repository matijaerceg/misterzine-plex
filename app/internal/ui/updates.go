package ui

import (
	"context"
	"fmt"
	"os"
	"plexcrt/internal/beta"
	"plexcrt/internal/gfx"
	"plexcrt/internal/input"
	"plexcrt/internal/updates"
	"time"
)

type updateState struct {
	catalogue  updates.Catalogue
	checking   bool
	lastCheck  time.Time
	nextStatus time.Time
	reading    bool
	status     updates.Status
	message    string
	launched   time.Time
}

func (a *App) checkUpdates(manual bool, now time.Time) {
	if a.Cfg == nil || a.betaDir() == "" || a.updates.checking {
		return
	}
	if !manual && !a.updates.lastCheck.IsZero() && now.Sub(a.updates.lastCheck) < 6*time.Hour {
		return
	}
	a.updates.checking = true
	a.updates.lastCheck = now
	if manual {
		a.updates.message = "Checking for updates..."
	}
	root := a.betaDir()
	go func() {
		cached, cacheErr := updates.Cached(root)
		var c updates.Catalogue
		var err error
		if file := os.Getenv("PLEXCRT_CATALOGUE_FILE"); file != "" {
			// a local catalogue stands in for the published one (testing)
			var data []byte
			if data, err = os.ReadFile(file); err == nil {
				c, err = updates.Parse(data)
			}
		} else {
			c, err = updates.Fetch(context.Background(), nil, updates.CatalogueURL)
		}
		if err == nil {
			_ = updates.SaveCache(root, c)
		}
		a.Later(func() {
			a.updates.checking = false
			if err != nil {
				if cacheErr == nil && a.updates.catalogue.Releases == nil {
					a.updates.catalogue = cached
				}
				if manual {
					a.updates.message = "Could not check for updates. Try again later."
				}
				return
			}
			a.updates.catalogue = c
			if manual {
				a.updates.message = "Release information is up to date."
			}
		})
	}()
}
func (a *App) pollUpdates(now time.Time) {
	if a.Cfg == nil || a.betaDir() == "" {
		return
	}
	a.checkUpdates(false, now)
	if a.updates.reading || now.Before(a.updates.nextStatus) {
		return
	}
	a.updates.reading = true
	a.updates.nextStatus = now.Add(time.Second)
	root := a.betaDir()
	go func() {
		s := updates.ReadStatus(root)
		a.Later(func() {
			a.updates.reading = false
			if !a.updates.launched.IsZero() && s.Updated < float64(a.updates.launched.UnixNano())/1e9 {
				if time.Since(a.updates.launched) < 10*time.Second {
					return
				}
				s = updates.Status{Stage: "failed", Message: "The updater did not start. Run Install to repair update support."}
			}
			a.updates.launched = time.Time{}
			a.updates.status = s
		})
	}()
}
func (a *App) updateAvailable() bool {
	if a.updates.status.Stage == "ready" {
		return true
	}
	if a.Cfg == nil {
		return false
	}
	for _, r := range a.updates.catalogue.Releases {
		dismissed := false
		for _, id := range a.Cfg.DismissedUpdates {
			if id == r.ID {
				dismissed = true
			}
		}
		if !dismissed && updates.Notify(r, a.Version, beta.Channel, a.Cfg.EarlyAccessUpdates) {
			return true
		}
	}
	return false
}
func (a *App) startUpdate(action string, r *updates.Release) {
	if a.updates.status.Busy() || !a.Starting.IsZero() {
		return
	}
	if err := updates.Start(a.betaDir(), action, r); err != nil {
		a.updates.message = "Could not start the updater. Run MisterZine-Plex-Install to repair update support."
		return
	}
	stage := "download"
	if action == "activate" {
		stage = "activating"
	}
	a.updates.launched = time.Now()
	a.updates.message = ""
	a.updates.status = updates.Status{Stage: stage, Message: "Starting...", Release: r}
}

// Updates is available on both public and beta builds; notifications are quieter
// than the full catalogue so public users are not repeatedly offered paid betas.
type Updates struct {
	app     *App
	cur     int
	release *updates.Release
	confirm bool
}
type updateAction struct {
	label string
	do    func()
}

func NewUpdates(a *App) *Updates { a.checkUpdates(true, time.Now()); return &Updates{app: a} }
func (u *Updates) actions() []updateAction {
	a := u.app
	if u.release != nil {
		r := *u.release
		if u.confirm {
			return []updateAction{
				{"Enter code and update", func() {
					b := NewBetaAccess(a, func() { u.release = nil; u.confirm = false; u.cur = 0; a.startUpdate("prepare", &r) })
					b.requirement = r.Requirement()
					b.version = r.Version
					a.Push(b)
				}},
				{"Install for browsing", func() { u.release = nil; u.confirm = false; u.cur = 0; a.startUpdate("prepare", &r) }},
				{"Cancel", func() { u.confirm = false; u.cur = 0 }},
			}
		}
		return []updateAction{
			{"Install release", func() {
				if r.Requirement().Check(a.betaDir()) != nil {
					u.confirm = true
					u.cur = 0
					return
				}
				u.release = nil
				u.cur = 0
				a.startUpdate("prepare", &r)
			}},
			{"Dismiss notification", func() {
				before := append([]string(nil), a.Cfg.DismissedUpdates...)
				a.Cfg.DismissedUpdates = append(a.Cfg.DismissedUpdates, r.ID)
				if len(a.Cfg.DismissedUpdates) > 32 {
					a.Cfg.DismissedUpdates = a.Cfg.DismissedUpdates[len(a.Cfg.DismissedUpdates)-32:]
				}
				if err := a.Cfg.Save(); err != nil {
					a.Cfg.DismissedUpdates = before
					a.updates.message = "Could not save notification preference."
				}
				u.release = nil
				u.cur = 0
			}},
			{"Back", func() { u.release = nil; u.cur = 0 }},
		}
	}
	label := "Early-access notifications: Off"
	if a.Cfg.EarlyAccessUpdates {
		label = "Early-access notifications: On"
	}
	actions := []updateAction{{"Check for updates", func() { a.checkUpdates(true, time.Now()) }}}
	if beta.Channel != "beta" {
		actions = append(actions, updateAction{label, func() {
			a.Cfg.EarlyAccessUpdates = !a.Cfg.EarlyAccessUpdates
			if err := a.Cfg.Save(); err != nil {
				a.Cfg.EarlyAccessUpdates = !a.Cfg.EarlyAccessUpdates
				a.updates.message = "Could not save notification preference."
			}
		}})
	}
	for _, channel := range []string{"public", "beta"} {
		if r, ok := a.updates.catalogue.Releases[channel]; ok {
			title := "Public: "
			if channel == "beta" {
				title = "Early access: "
			}
			if r.ID == a.Build {
				title += "Installed - "
			}
			actions = append(actions, updateAction{title + r.Version, func() { u.release = &r; u.cur = 0 }})
		}
	}
	if a.updates.status.Stage == "ready" {
		actions = append(actions, updateAction{"Restart now", func() { a.startUpdate("activate", nil) }}, updateAction{"Later", func() { a.Pop() }})
	}
	return actions
}
func (u *Updates) Back() bool {
	if u.release != nil {
		u.release = nil
		u.confirm = false
		u.cur = 0
		return true
	}
	u.app.Pop()
	return true
}
func (u *Updates) Key(ev input.Event, now time.Time) {
	if ev.Release {
		return
	}
	actions := u.actions()
	switch ev.Key {
	case input.Up:
		u.cur = max(0, u.cur-1)
	case input.Down:
		u.cur = min(len(actions)-1, u.cur+1)
	case input.Enter:
		if !ev.Repeat && !u.app.updates.status.Busy() {
			actions[min(u.cur, len(actions)-1)].do()
		}
	}
}
func (u *Updates) Draw(c *gfx.Canvas, now time.Time) bool {
	c.Fill(0, 0, c.W, c.H, gfx.Bg)
	a := u.app
	f := a.F
	a.text(c, MenuX, SafeY, f.Title, gfx.White, "Updates")
	y := 128
	if u.release != nil {
		r := u.release
		a.text(c, MenuX, 82, f.Body, gfx.Amber, f.Body.Fit(r.Version+" - "+r.Channel, MenuWidth))
		if u.confirm {
			for i, line := range []string{"This release requires a new Patreon code.", "Your current version will keep working."} {
				a.text(c, MenuX, 136+i*30, f.Body, gfx.White, line)
			}
			a.text(c, MenuX, 209, f.Body, gfx.Purple, patreonAddress)
		} else {
			a.text(c, MenuX, 114, f.SmallBold, gfx.GreyLo, fmt.Sprintf("Download: %.1f MB", float64(r.Size)/(1024*1024)))
			for i, line := range wrap(f.Body, r.Notes, MenuWidth, 4) {
				a.text(c, MenuX, 151+i*26, f.Body, gfx.GreyHi, line)
			}
		}
		y = 304
	} else {
		a.text(c, MenuX, 82, f.SmallBold, gfx.GreyLo, f.SmallBold.Fit("Installed: "+a.Version+" - "+beta.Channel, MenuWidth))
	}
	actions := u.actions()
	u.cur = min(u.cur, len(actions)-1)
	for i, it := range actions {
		col := gfx.GreyHi
		if i == u.cur {
			col = gfx.White
			menuFocusBar(c, MenuX, y+9, f.Body.Height())
		}
		a.text(c, MenuX, y+9, f.Body, col, f.Body.Fit(it.label, MenuWidth))
		y += 32
	}
	if u.release == nil {
		// The worker's outcome owns this area: a failure stays readable until the
		// next update starts, and a catalogue check result never covers it.
		s := a.updates.status
		note := a.updates.message
		if a.updates.checking {
			note = "Checking for updates..."
		}
		const lines = 4
		y := 352
		if note != "" {
			for _, line := range wrap(f.SmallBold, note, MenuWidth, 1) {
				a.text(c, MenuX, y, f.SmallBold, gfx.GreyLo, line)
				y += 23
			}
		}
		if s.Stage != "" {
			message := s.Message
			if message == "" {
				message = s.Stage
			}
			if s.Stage == "ready" {
				message = "The update is downloaded but not installed. Choose Restart now to install it."
				if s.Release != nil {
					message = s.Release.Version + " is downloaded but not installed. Choose Restart now to install it."
				}
			}
			room := lines - (y-352)/23
			if s.Detail != "" && room > 1 {
				room--
			}
			for _, line := range wrap(f.SmallBold, message, MenuWidth, room) {
				a.text(c, MenuX, y, f.SmallBold, gfx.Purple, line)
				y += 23
			}
			if s.Detail != "" && (y-352)/23 < lines {
				for _, line := range wrap(f.SmallBold, s.Detail, MenuWidth, 1) {
					a.text(c, MenuX, y, f.SmallBold, gfx.GreyLo, line)
				}
			}
		}
	}
	return false
}
