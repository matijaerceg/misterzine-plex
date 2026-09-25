package ring

import "testing"

func TestDotRunKeepsTheLoadBit(t *testing.T) {
	r := &Ring{hdr: new([32]uint32)}
	o := &Overlay{r: r}
	loadBit := func() uint32 { return r.hdr[25] >> 28 & 1 }
	o.Dot(320, 240, 0xE5A00D, true) // a new place: the load bit toggles
	o.DotRun(2, 320, 400)
	if loadBit() != 1 {
		t.Fatalf("DotRun dropped the load bit Dot set: run word %x", r.hdr[25])
	}
	if xmin, xmax, vx := r.hdr[25]&0x3FF, r.hdr[25]>>10&0x3FF, int8(r.hdr[25]>>20); xmin != 320 || xmax != 400 || vx != 2 {
		t.Fatalf("run word %x: xmin %d xmax %d vx %d", r.hdr[25], xmin, xmax, vx)
	}
	o.Dot(330, 240, 0xE5A00D, true) // toggles back
	o.DotRun(0, 57, 663)            // an odd limit: its low bit is not the load bit
	o.DotRun(-2, 320, 400)
	if loadBit() != 0 {
		t.Fatalf("DotRun set a load bit Dot had cleared: run word %x", r.hdr[25])
	}
	if vx := int8(r.hdr[25] >> 20); vx != -2 {
		t.Fatalf("turning round: run word %x", r.hdr[25])
	}
}

func TestForeignNeedsAFrame(t *testing.T) {
	r := &Ring{hdr: new([32]uint32)}
	r.hdr[0], r.hdr[1], r.seq = magic, 7, 7
	if r.Foreign() {
		t.Fatal("our own frame counted as the presenter's")
	}
	r.hdr[0], r.hdr[1] = 0, 0 // a framebuffer mode write wiped the header
	if r.Foreign() {
		t.Fatal("a wiped header counted as the presenter's frame")
	}
	r.hdr[0], r.hdr[1] = magic, 8
	if !r.Foreign() {
		t.Fatal("the presenter's frame was missed")
	}
}
