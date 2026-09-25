package ring

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Forensics for a ring that gets overwritten on a board we cannot see.
//
// The words between the header block and the first slot are never touched
// by the app, the presenter or the core, so the watchdog plants canaries
// there. When the header is found wiped, the canaries say what happened: a
// clear of the whole framebuffer zeroes them, a process painting into the
// memory leaves its own pixel values, and a write aimed at the header alone
// leaves them intact. The mapper scan then names the processes able to
// write this memory at that moment.

// canaryOffs lie in the gap 0x80..0x100000: past the header, status and
// sprite words, below slot 0.
var canaryOffs = [...]int{0x1000, 0x10000, 0x80000, 0xFF000}

func canaryValue(i int) uint32 { return 0xC0DE5A00 | uint32(i) }

func (r *Ring) word(off int) *uint32 { return (*uint32)(unsafe.Pointer(&r.mem[off])) }

func (r *Ring) plantCanaries() {
	for i, off := range canaryOffs {
		atomic.StoreUint32(r.word(off), canaryValue(i))
	}
}

// canaryState reports how many canaries changed, whether every changed one
// is zero, and the value of the first changed one.
func (r *Ring) canaryState() (hit int, zeroed bool, first uint32) {
	zeroed = true
	for i, off := range canaryOffs {
		v := atomic.LoadUint32(r.word(off))
		if v == canaryValue(i) {
			continue
		}
		if hit == 0 {
			first = v
		}
		hit++
		if v != 0 {
			zeroed = false
		}
	}
	return hit, zeroed, first
}

func describeCanaries(hit int, zeroed bool, first uint32) string {
	switch {
	case hit == 0:
		return "intact (the write was aimed at the header)"
	case zeroed:
		return fmt.Sprintf("zeroed, %d of %d (a clear of the framebuffer)", hit, len(canaryOffs))
	default:
		return fmt.Sprintf("overwritten, %d of %d, first now %08x (something painting into the framebuffer)", hit, len(canaryOffs), first)
	}
}

// headerWords is the first few header words as found, for the log.
func (r *Ring) headerWords() string {
	return fmt.Sprintf("%08x %08x %08x %08x", atomic.LoadUint32(&r.hdr[0]), atomic.LoadUint32(&r.hdr[1]),
		atomic.LoadUint32(&r.hdr[2]), atomic.LoadUint32(&r.hdr[3]))
}

// ringPhys is the physical address of the ring: the framebuffer's start as
// the driver reports it, else the address the presenter maps.
func ringPhys() (phys uint64, fromDriver bool) {
	f, err := os.Open("/dev/fb0")
	if err != nil {
		return 0x30000000, false
	}
	defer f.Close()
	var fix [128]byte // struct fb_fix_screeninfo
	const fbiogetFscreeninfo = 0x4602
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), fbiogetFscreeninfo, uintptr(unsafe.Pointer(&fix[0]))); e != 0 {
		return 0x30000000, false
	}
	// id[16], then unsigned long smem_start
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return *(*uint64)(unsafe.Pointer(&fix[16])), true
	}
	return uint64(*(*uint32)(unsafe.Pointer(&fix[16]))), true
}

// mappers lists the processes other than this one that can write the ring's
// memory: an open /dev/fb0, a mapping of it, or a /dev/mem mapping that
// covers the ring's physical range.
func mappers(proc string, phys, size uint64, self int) []string {
	entries, _ := os.ReadDir(proc)
	var out []string
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		dir := proc + "/" + e.Name()
		var fd, fbmap, memrw, memro bool
		if fds, err := os.ReadDir(dir + "/fd"); err == nil {
			for _, x := range fds {
				if t, err := os.Readlink(dir + "/fd/" + x.Name()); err == nil && t == "/dev/fb0" {
					fd = true
					break
				}
			}
		}
		if f, err := os.Open(dir + "/maps"); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				fields := strings.Fields(sc.Text())
				if len(fields) < 6 {
					continue
				}
				switch fields[5] {
				case "/dev/fb0":
					fbmap = true
				case "/dev/mem":
					span := strings.SplitN(fields[0], "-", 2)
					if len(span) != 2 {
						continue
					}
					lo, e1 := strconv.ParseUint(span[0], 16, 64)
					hi, e2 := strconv.ParseUint(span[1], 16, 64)
					off, e3 := strconv.ParseUint(fields[2], 16, 64) // the physical address
					if e1 != nil || e2 != nil || e3 != nil || hi <= lo {
						continue
					}
					if off < phys+size && off+(hi-lo) > phys {
						if strings.Contains(fields[1], "w") {
							memrw = true
						} else {
							memro = true
						}
					}
				}
			}
			f.Close()
		}
		var kinds []string
		for _, k := range []struct {
			on   bool
			name string
		}{{fd, "fd"}, {fbmap, "fb0-map"}, {memrw, "mem-rw"}, {memro, "mem-ro"}} {
			if k.on {
				kinds = append(kinds, k.name)
			}
		}
		if len(kinds) == 0 {
			continue
		}
		comm, _ := os.ReadFile(dir + "/comm")
		out = append(out, fmt.Sprintf("%s(%d) %s", strings.TrimSpace(string(comm)), pid, strings.Join(kinds, "+")))
	}
	return out
}

func listOrNone(names []string) string {
	if len(names) == 0 {
		return "no other process"
	}
	return strings.Join(names, ", ")
}
