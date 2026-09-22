package safelog

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitCredentialRedaction(t *testing.T) {
	var b bytes.Buffer
	w := &Writer{Out: &b, Secrets: []string{"private+/token"}}
	for _, s := range []string{"failed https://server/?X-Plex-To", "ken=private%2B%2Ftoken&x=1\n", "Authorization: Bearer abc123\n", "private+/token"} {
		w.Write([]byte(s))
	}
	w.Flush()
	for _, secret := range []string{"private", "abc123"} {
		if strings.Contains(b.String(), secret) {
			t.Fatalf("credential escaped redaction: %s", secret)
		}
	}
	if !strings.Contains(b.String(), "&x=1") {
		t.Fatal("lost useful error context")
	}
}
func TestOversizedLineDropped(t *testing.T) {
	var b bytes.Buffer
	w := &Writer{Out: &b}
	w.Write([]byte(strings.Repeat("x", 70000)))
	w.Write([]byte("token=secret\n"))
	if strings.Contains(b.String(), "secret") || strings.Contains(b.String(), "xxx") {
		t.Fatal("oversized line leaked")
	}
}
