//go:build windows

package benchmark

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	modKernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetProcessTimes = modKernel32.NewProc("GetProcessTimes")
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

	var creation, exit, kernel, user syscall.Filetime
	handle, _ := syscall.GetCurrentProcess()
	_, _, _ = procGetProcessTimes.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)

	return resourceSnapshot{
		userMS:  filetimeToMS(user),
		sysMS:   filetimeToMS(kernel),
		allocMB: float64(mem.Alloc) / 1024.0 / 1024.0,
		sysMB:   float64(mem.Sys) / 1024.0 / 1024.0,
	}
}

func filetimeToMS(ft syscall.Filetime) int64 {
	value := (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
	return int64(value / 10000)
}
