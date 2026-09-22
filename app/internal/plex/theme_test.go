package plex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func TestThemeMetadataCacheAndCancellation(t *testing.T) {
	var downloads atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" || r.URL.RawQuery != "" {
			t.Error("authentication must stay in headers")
		}
		if r.URL.Path == "/library/metadata/1" {
			fmt.Fprint(w, `<MediaContainer><Directory theme="/library/metadata/1/theme/123"/></MediaContainer>`)
			return
		}
		downloads.Add(1)
		fmt.Fprint(w, "theme audio")
	}))
	defer s.Close()
	c := New(s.URL, "test-token", t.TempDir(), "test")
	f, err := c.ThemeFile(context.Background(), "1", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f)
	if err != nil || string(data) != "theme audio" {
		t.Fatal("cache not written")
	}
	if _, err = c.ThemeFile(context.Background(), "1", ""); err != nil {
		t.Fatal(err)
	}
	if downloads.Load() != 1 {
		t.Fatal("cached theme downloaded again")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = c.ThemeFile(ctx, "2", ""); err == nil {
		t.Fatal("cancelled request succeeded")
	}
}
