package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"plexcrt/internal/beta"
	"plexcrt/internal/input"
)

func TestForgetBetaAccess(t *testing.T) {
	a := betaTestApp(t)
	dir := a.betaDir()
	if err := beta.Unlock(dir, "012345"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"beta-unlocks/older.receipt", "beta-keys/older.key", "settings.json", "cache/art"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	a.chooseBetaAccess()
	a.top().(*Chooser).Key(input.Event{Key: input.Enter}, now)
	s := a.top().(*ForgetBetaAccess)
	s.Key(input.Event{Key: input.Enter}, now)
	if beta.Check(dir) != nil {
		t.Fatal("cancel removed access")
	}
	a.chooseBetaAccess()
	a.top().(*Chooser).Key(input.Event{Key: input.Enter}, now)
	s = a.top().(*ForgetBetaAccess)
	s.Key(input.Event{Key: input.Down}, now)
	s.Key(input.Event{Key: input.Enter, Repeat: true}, now)
	if beta.Check(dir) != nil {
		t.Fatal("repeat removed access")
	}
	s.Key(input.Event{Key: input.Enter}, now)
	if beta.Check(dir) != beta.ErrLocked {
		t.Fatal("playback still unlocked")
	}
	for _, name := range []string{"beta-unlocks", "beta-keys"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", name, err)
		}
	}
	for _, name := range []string{"settings.json", "cache/art"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != "keep" {
			t.Fatalf("%s changed: %v", name, err)
		}
	}
	if err := beta.Unlock(dir, "012345"); err != nil {
		t.Fatal(err)
	}
	if beta.Check(dir) != nil {
		t.Fatal("cannot unlock again")
	}
}
