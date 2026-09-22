package ring

import (
	"sync"
	"sync/atomic"
	"time"
)

// VideoControl publishes a leased request separately from frame and overlay
// producers. The timer runs even if the UI is waiting for a network request.
type VideoControl struct {
	mu             sync.Mutex
	mode, previous uint32
	deadline       time.Time
}

func (r *Ring) StartVideo(mode uint32) func() {
	r.SetVideo(mode)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			r.video.mu.Lock()
			if !r.video.deadline.IsZero() && !time.Now().Before(r.video.deadline) {
				r.video.mode = r.video.previous
				r.video.deadline = time.Time{}
			}
			mode := r.video.mode
			r.video.mu.Unlock()
			seq := (atomic.LoadUint32(&r.hdr[28]) + 2) &^ 1
			atomic.StoreUint32(&r.hdr[28], seq|1)
			atomic.StoreUint32(&r.hdr[29], 0x56500000|mode)
			atomic.StoreUint32(&r.hdr[30], seq)
			atomic.StoreUint32(&r.hdr[28], seq)
			select {
			case <-tick.C:
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop); <-done; atomic.StoreUint32(&r.hdr[29], 0) }
}

// SetVideo restores a known preference without starting another trial.
func (r *Ring) SetVideo(mode uint32) {
	r.video.mu.Lock()
	defer r.video.mu.Unlock()
	if mode != 2 {
		mode = 0
	}
	r.video.mode = mode
	r.video.deadline = time.Time{}
}

func (r *Ring) VideoStatus() (mode uint32, override, supported bool) {
	v := atomic.LoadUint32(&r.hdr[27])
	return v & 3, v&4 != 0, v&0xfffffff0 == 0x56500000
}

// VideoLocked reports the core's active CRT-profile safety interlock.
func (r *Ring) VideoLocked() bool {
	v := atomic.LoadUint32(&r.hdr[27])
	return v&0xfffffff0 == 0x56500000 && v&8 != 0
}

// Tap is read by the core each scanline and goes straight to its audio mixer.
func (r *Ring) Tap() {
	atomic.AddUint32(&r.hdr[31], 1)
}

func (r *Ring) TryVideo(mode uint32) time.Time {
	r.video.mu.Lock()
	defer r.video.mu.Unlock()
	r.video.previous = r.video.mode
	r.video.mode = mode
	r.video.deadline = time.Now().Add(15 * time.Second)
	return r.video.deadline
}

func (r *Ring) FinishVideo(keep bool) bool {
	r.video.mu.Lock()
	defer r.video.mu.Unlock()
	valid := !r.video.deadline.IsZero() && time.Now().Before(r.video.deadline)
	if !valid {
		return false
	}
	if r.VideoLocked() {
		keep = false
	}
	if !keep {
		r.video.mode = r.video.previous
	}
	r.video.deadline = time.Time{}
	return keep
}
