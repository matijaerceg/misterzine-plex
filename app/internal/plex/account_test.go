package plex

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type accountTransport func(*http.Request) (*http.Response, error)

func (f accountTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAccountName(t *testing.T) {
	old := tvClient
	t.Cleanup(func() { tvClient = old })
	for _, tc := range []struct {
		body   string
		status int
		want   string
		bad    bool
	}{
		{`{"username":" alice ","title":"Alice"}`, 200, "alice", false},
		{`{"username":"","title":" Alice "}`, 200, "Alice", false},
		{`{"email":"private@example.com"}`, 200, "", false},
		{`{broken`, 200, "", true},
		{`{}`, 401, "", true},
		{`{}`, 503, "", true},
	} {
		tvClient = &http.Client{Transport: accountTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v2/user" || r.Header.Get("X-Plex-Token") != "account-token" || r.Header.Get("X-Plex-Client-Identifier") != "client" {
				t.Fatal("incorrect account request")
			}
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		name, err := AccountName("client", "account-token")
		if name != tc.want || (err != nil) != tc.bad {
			t.Fatalf("status %d body %s: name=%q err=%v", tc.status, tc.body, name, err)
		}
	}
}
