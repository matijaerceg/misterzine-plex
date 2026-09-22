package input

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Sim reads key names from a FIFO, one per line ("down", "enter", ...), for
// scripted tests without a pad: echo down > /tmp/plexcrt.ctl. "hold:down:20"
// simulates holding down through 20 auto-repeats.
func Sim(path string, stop <-chan struct{}) <-chan Event {
	out := make(chan Event, 64)
	os.Remove(path)
	if err := syscall.Mkfifo(path, 0o666); err != nil {
		return out
	}
	go func() {
		keyboard := newKeyboard()
		for {
			f, err := os.Open(path) // blocks until a writer appears
			if err != nil {
				return
			}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				for _, w := range strings.Fields(sc.Text()) {
					// Private testing: raw set-2 make/break words use the same decoder
					// as physical input, e.g. scan:21c then scan:01c for A.
					if strings.HasPrefix(w, "scan:") {
						if word, err := strconv.ParseUint(strings.TrimPrefix(w, "scan:"), 16, 32); err == nil {
							if ev, ok := keyboard.decode(uint32(word)); ok {
								out <- ev
							}
						}
						continue
					}
					reps := 0
					if strings.HasPrefix(w, "hold:") {
						parts := strings.Split(w, ":")
						w = parts[1]
						if len(parts) > 2 {
							reps, _ = strconv.Atoi(parts[2])
						}
					}
					for k := Key(0); k < None; k++ {
						if names[k] != w {
							continue
						}
						out <- Event{Key: k}
						for i := 1; i <= reps; i++ {
							time.Sleep(repeatRate)
							out <- Event{Key: k, Repeat: true, Count: i}
						}
						out <- Event{Key: k, Release: true, Count: reps}
					}
				}
			}
			f.Close()
		}
	}()
	return out
}
