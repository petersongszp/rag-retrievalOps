//go:build !windows

package benchmark

import (
	"runtime"
	"syscall"
)

type resourceSnapshot struct {
	userMS  int64
	sysMS   int64
	allocMB float64
	sysMB   float64
}

func captureResourceSnapshot() resourceSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)

	return resourceSnapshot{
		userMS:  timevalMilliseconds(usage.Utime),
		sysMS:   timevalMilliseconds(usage.Stime),
		allocMB: float64(mem.Alloc) / 1024.0 / 1024.0,
		sysMB:   float64(mem.Sys) / 1024.0 / 1024.0,
	}
}

func timevalMilliseconds(tv syscall.Timeval) int64 {
	return tv.Sec*1000 + int64(tv.Usec)/1000
}
