package ring

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestVideoRollbackAndLease(t *testing.T) {
	r := &Ring{hdr: new([32]uint32)}
	stop := r.StartVideo(0)
	defer stop()
	r.TryVideo(2)
	r.video.mu.Lock()
	r.video.deadline = time.Now().Add(40 * time.Millisecond)
	r.video.mu.Unlock()
	time.Sleep(350 * time.Millisecond)
	if v := atomic.LoadUint32(&r.hdr[29]); v != 0x56500000 {
		t.Fatalf("rollback failed: %x", v)
	}
	if r.FinishVideo(true) {
		t.Fatal("expired trial was confirmed")
	}
	r.TryVideo(2)
	if !r.FinishVideo(true) {
		t.Fatal("live trial not confirmed")
	}
	time.Sleep(300 * time.Millisecond)
	if atomic.LoadUint32(&r.hdr[29]) != 0x56500002 {
		t.Fatal("confirmation did not stick")
	}
	a, b := atomic.LoadUint32(&r.hdr[28]), atomic.LoadUint32(&r.hdr[30])
	if a != b || a&1 != 0 {
		t.Fatal("invalid seqlock")
	}
}

func TestCRTProfileStatusAndConfirmation(t *testing.T) {
	r := &Ring{hdr: new([32]uint32)}
	for _, status := range []uint32{0x56500008, 0x5650000c} {
		atomic.StoreUint32(&r.hdr[27], status)
		mode, override, supported := r.VideoStatus()
		if !r.VideoLocked() || !supported || mode != 0 || override != (status&4 != 0) {
			t.Fatal("CRT status was misread")
		}
		r.TryVideo(2)
		if r.FinishVideo(true) {
			t.Fatal("CRT profile accepted 480p confirmation")
		}
	}
	atomic.StoreUint32(&r.hdr[27], 0x56500002)
	if r.VideoLocked() {
		t.Fatal("HDMI profile stayed locked")
	}
	r.TryVideo(2)
	if !r.FinishVideo(true) {
		t.Fatal("HDMI confirmation rejected")
	}
}
