package ring

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Watch reports, in the log, the things a healthy ring never shows: the
// header rewritten by someone else, the Linux framebuffer mode changed under
// the app, or the slot the app just published reading back black. It exists
// so that a flickering or black screen on a board we cannot see explains
// itself in one diagnostics report. When the header is found lost, wake is
// called so the app redraws at once: MiSTer main wipes this memory whenever
// the framebuffer mode is written, and a redraw is what brings the picture
// back. It stops when stop is closed.
func (r *Ring) Watch(logf func(string, ...any), wake func(), stop <-chan struct{}) {
	const modeFile = "/sys/module/MiSTer_fb/parameters/mode"
	readMode := func() string {
		b, err := os.ReadFile(modeFile)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	logf("watch: framebuffer mode %q, console %s", readMode(), consoleState())
	mode := readMode()
	var lostHeader, foreign, black int
	var lastSeq uint32
	report := time.Now()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for n := 0; ; n++ {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		if atomic.LoadUint32(&r.hdr[0]) != magic && atomic.LoadUint32(&r.pubSeq) != 0 {
			lostHeader++
			if wake != nil {
				wake()
			}
		}
		// Our own publish moves seq between two samples; a stranger's leaves
		// it away from ours across two consecutive samples.
		// Nothing to judge before the app's first publish: the header and
		// the slots still hold whatever the previous process left.
		seq := atomic.LoadUint32(&r.hdr[1])
		if ours := atomic.LoadUint32(&r.pubSeq); ours != 0 {
			if seq != ours && seq == lastSeq {
				foreign++
			}
			if r.publishedBlack() {
				black++
			}
		}
		lastSeq = seq
		if n%10 == 0 {
			if m := readMode(); m != mode {
				logf("watch: framebuffer mode changed from %q to %q", mode, m)
				mode = m
			}
		}
		if time.Since(report) >= 5*time.Second {
			if lostHeader+foreign+black > 0 {
				logf("watch: in 5 s the header was found wiped %d times, published by another writer %d times, and the published slot read black %d times",
					lostHeader, foreign, black)
			}
			lostHeader, foreign, black = 0, 0, 0
			report = time.Now()
		}
	}
}

// publishedBlack samples a handful of pixels of the published slot. Reading
// write-combined memory is slow, so this is a few words, not a frame.
func (r *Ring) publishedBlack() bool {
	slot := atomic.LoadUint32(&r.hdr[2]) % NSlot
	off := slot0 + int(slot)*slotSize
	for _, p := range [...]int{W*40 + 40, W*120 + 360, W*240 + 100, W*240 + 620, W*360 + 360, W*440 + 680} {
		i := off + p*4
		if r.mem[i]|r.mem[i+1]|r.mem[i+2] != 0 {
			return false
		}
	}
	return true
}

func consoleState() string {
	var parts []string
	if b, err := os.ReadFile("/sys/class/graphics/fb0/state"); err == nil {
		parts = append(parts, "fb0 state "+strings.TrimSpace(string(b)))
	}
	entries, _ := os.ReadDir("/sys/class/vtconsole")
	for _, e := range entries {
		if b, err := os.ReadFile("/sys/class/vtconsole/" + e.Name() + "/bind"); err == nil {
			parts = append(parts, fmt.Sprintf("%s bound %s", e.Name(), strings.TrimSpace(string(b))))
		}
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, ", ")
}
