package ui

import (
	"plexcrt/internal/beta"
	"plexcrt/internal/input"
	"plexcrt/internal/updates"
	"testing"
	"time"
)

func TestTargetCodeDoesNotUnlockCurrentRelease(t *testing.T) {
	a := betaTestApp(t)
	r := updates.Release{Version: "0.3.0-beta.1", Channel: "beta", Access: &updates.Access{Batch: "next-fixture", SHA256: beta.CodeSHA256}}
	u := &Updates{app: a, release: &r, confirm: true}
	a.Push(u)
	u.actions()[0].do()
	b := a.top().(*BetaAccess)
	now := time.Now()
	enterFixture(a, now)
	if b.opened.IsZero() || r.Requirement().Check(a.betaDir()) != nil {
		t.Fatal("target receipt was not saved")
	}
	if beta.Check(a.betaDir()) == nil {
		t.Fatal("target code unlocked current access batch")
	}
	// Stop before completion; no update worker should be launched by this fixture.
}
func TestUpdateChoicesAndCancel(t *testing.T) {
	a := betaTestApp(t)
	u := &Updates{app: a}
	if len(u.actions()) != 1 {
		t.Fatal("unpublished channels shown")
	}
	r := updates.Release{ID: "next", Version: "0.3.0-beta.1", Channel: "beta", Access: &updates.Access{Batch: "next", SHA256: beta.CodeSHA256}}
	a.updates.catalogue = updates.Catalogue{Schema: 1, Releases: map[string]updates.Release{"beta": r}}
	if !a.updateAvailable() {
		t.Fatal("new beta not indicated")
	}
	a.Cfg.DismissedUpdates = []string{"next"}
	if a.updateAvailable() {
		t.Fatal("dismissal ignored")
	}
	u.release = &r
	u.actions()[0].do()
	if !u.confirm {
		t.Fatal("new code confirmation missing")
	}
	u.cur = 2
	u.Key(input.Event{Key: input.Enter}, time.Now())
	if u.confirm || a.updates.status.Busy() {
		t.Fatal("cancel started update")
	}
}
