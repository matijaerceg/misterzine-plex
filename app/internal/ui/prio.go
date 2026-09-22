package ui

import (
	"os"
	"strconv"
	"syscall"
)

// raisePriority moves every thread of this process to the given nice
// value (negative is higher) and returns a function that puts them back.
// Linux applies setpriority per thread, so /proc/self/task is walked;
// threads made later inherit from their maker. Processes started before
// the call keep their own priority.
func raisePriority(nice int) func() {
	for _, tid := range threads() {
		syscall.Setpriority(syscall.PRIO_PROCESS, tid, nice)
	}
	return func() {
		for _, tid := range threads() {
			syscall.Setpriority(syscall.PRIO_PROCESS, tid, 0)
		}
	}
}

func threads() []int {
	ents, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return []int{0}
	}
	var out []int
	for _, e := range ents {
		if n, err := strconv.Atoi(e.Name()); err == nil {
			out = append(out, n)
		}
	}
	return out
}
