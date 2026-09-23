package ui

import (
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type accountRoundTrip func(*http.Request) (*http.Response, error)

func (f accountRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAccountNameLookupLifecycle(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, scenario := range []string{"existing login", "signed out", "changed account", "offline"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := LoadConfig(filepath.Join(t.TempDir(), "settings.json"))
			cfg.Token, cfg.ServerToken, cfg.ServerURL = "account-token", "server-token", "http://server"
			a := &App{Cfg: cfg, Wake: make(chan struct{}, 1), later: make(chan func(), 1), Log: log.New(io.Discard, "", 0)}
			http.DefaultTransport = accountRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("X-Plex-Token") != "account-token" {
					t.Error("used server credentials")
				}
				status := 200
				if scenario == "offline" {
					status = 503
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"username":"Alice"}`))}, nil
			})
			a.loadAccountName()
			switch scenario {
			case "signed out":
				if err := cfg.SignOut(); err != nil {
					t.Fatal(err)
				}
			case "changed account":
				cfg.Token = "different-account"
			}
			select {
			case <-a.Wake:
			case <-time.After(2 * time.Second):
				t.Fatal("lookup did not finish")
			}
			a.runLater()
			if scenario == "existing login" {
				if cfg.AccountName != "Alice" || LoadConfig(cfg.path).AccountName != "Alice" {
					t.Fatal("name not saved")
				}
			} else if cfg.AccountName != "" {
				t.Fatal("stale or failed lookup saved name")
			}
			if scenario == "offline" && (!cfg.SignedIn() || cfg.Token != "account-token") {
				t.Fatal("failed lookup changed sign-in")
			}
		})
	}
}

func TestSignOutLabelUsesAccount(t *testing.T) {
	for _, tc := range []struct{ token, name, want string }{
		{"account-token", "Alice", "Sign out (Alice)"},
		{"account-token", "", "Sign out"},
		{"", "", "Sign out"},
		{"", "stale name", "Sign out"},
	} {
		o := &Options{app: &App{Cfg: &Config{Token: tc.token, AccountName: tc.name, ServerName: "Movie Server"}}}
		items := o.items()
		if got := items[len(items)-3].label; got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}
