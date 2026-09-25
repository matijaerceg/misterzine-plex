package ring

import "sync/atomic"

// The core scales every pixel it sends by level/256, after the overlay and
// the sprites, taking the word at each vsync (rtl/ddr_scanout.v). The tag
// makes a wiped ring, which reads 0, full brightness rather than black.
const (
	Full      = 256 // full brightness
	brightOff = 0x88
	brightTag = 0x444D0000
)

// SetBrightness asks the core for level/256 (clamped to 0..Full) from the
// next field. The video lease rewrites it with the mode, and the watchdog
// after a wipe, so a cleared ring loses it only briefly.
func (r *Ring) SetBrightness(level int) {
	level = max(0, min(level, Full))
	atomic.StoreUint32(&r.dim, uint32(Full-level)) // zero value: full
	r.writeBrightness()
}

func (r *Ring) writeBrightness() {
	if len(r.mem) < brightOff+4 {
		return
	}
	atomic.StoreUint32(r.word(brightOff), brightTag|(Full-atomic.LoadUint32(&r.dim)))
}

// clearBrightness leaves the word untagged: full brightness for whoever
// shows the ring next.
func (r *Ring) clearBrightness() {
	if len(r.mem) < brightOff+4 {
		return
	}
	atomic.StoreUint32(r.word(brightOff), 0)
}

// Brightness is the level the core is using, and whether it dims at all:
// an older core leaves these bits of the dot word zero.
func (r *Ring) Brightness() (level int, supported bool) {
	v := atomic.LoadUint32(&r.hdr[26]) >> 16 & 0x1FF
	return int(v), v != 0
}
