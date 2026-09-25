package ring

import (
	"os"
	"strconv"
	"strings"
)

// MiSTer main sizes the Linux framebuffer for its own HDMI mode: at 720p, or
// with direct video, /dev/fb0 maps less than the ring. Main writes that mode
// again whenever it sets a video mode, from a worker thread, so the
// launcher's 1920x1080 can be undone a second after it. Every mapping of the
// ring is made with the mode large enough, and Watch puts it back when it
// shrinks. Writing the mode clears the ring's memory; the app redraws.

var modeFile = "/sys/module/MiSTer_fb/parameters/mode"

const fullMode = "8888 1 1920 1080 7680"

func readMode() string {
	b, err := os.ReadFile(modeFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// modeTooSmall reports whether a MiSTer_fb mode maps less than the ring. A
// mode it cannot read is left alone.
func modeTooSmall(mode string) bool {
	f := strings.Fields(mode)
	if len(f) != 5 {
		return false
	}
	var v [5]int
	for i, s := range f {
		n, err := strconv.Atoi(s)
		if err != nil {
			return false
		}
		v[i] = n
	}
	return v[0] != 8888 || v[3]*v[4] < mapSize
}

// enlargeMode writes the 1920x1080 mode when the current one maps less than
// the ring, and returns the mode it replaced ("" when it wrote nothing).
func enlargeMode() (string, error) {
	m := readMode()
	if !modeTooSmall(m) {
		return "", nil
	}
	return m, os.WriteFile(modeFile, []byte(fullMode+"\n"), 0)
}
