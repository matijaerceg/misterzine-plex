package ring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeTooSmall(t *testing.T) {
	for mode, want := range map[string]bool{
		"8888 1 1920 1080 7680": false,
		"8888 1 2048 1024 8192": false, // 8 MB, as large as the ring needs
		"8888 1 1280 720 5120":  true,  // a 720p HDMI mode
		"8888 1 720 480 2880":   true,  // direct video
		"565 1 1920 1080 3840":  true,
		"":                      false, // no MiSTer_fb: leave it alone
		"8888 1 1920 1080":      false,
		"8888 1 1920 x 7680":    false,
	} {
		if got := modeTooSmall(mode); got != want {
			t.Errorf("modeTooSmall(%q) = %v, want %v", mode, got, want)
		}
	}
}

func TestEnlargeModeWritesOnlyWhenTooSmall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode")
	defer func(old string) { modeFile = old }(modeFile)
	modeFile = path

	os.WriteFile(path, []byte("8888 1 1280 720 5120\n"), 0o644)
	from, err := enlargeMode()
	if err != nil || from != "8888 1 1280 720 5120" {
		t.Fatalf("enlargeMode() = %q, %v", from, err)
	}
	if got := readMode(); got != fullMode {
		t.Fatalf("mode after enlarging = %q", got)
	}

	// A large enough mode is never rewritten: each write clears the ring.
	os.WriteFile(path, []byte("8888 1 2048 1024 8192\n"), 0o644)
	if from, err := enlargeMode(); from != "" || err != nil {
		t.Fatalf("enlargeMode() on a large mode = %q, %v", from, err)
	}
	if got := readMode(); got != "8888 1 2048 1024 8192" {
		t.Fatalf("large mode rewritten to %q", got)
	}
}

func TestKeepRingMappedLogsTheChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode")
	defer func(old string) { modeFile = old }(modeFile)
	modeFile = path
	var logged []string
	logf := func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }

	os.WriteFile(path, []byte(fullMode+"\n"), 0o644)
	if got := keepRingMapped(logf, fullMode); got != fullMode || len(logged) != 0 {
		t.Fatalf("large mode: got %q, logged %q", got, logged)
	}
	os.WriteFile(path, []byte("8888 1 1280 720 5120\n"), 0o644)
	if got := keepRingMapped(logf, "8888 1 1280 720 5120"); got != fullMode {
		t.Fatalf("small mode: now %q", got)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], `"8888 1 1280 720 5120" maps less than the ring`) {
		t.Fatalf("logged %q", logged)
	}
}
