//go:build !windows

package benchmark

import (
	"syscall"
	"testing"
)

func TestTimevalMilliseconds(t *testing.T) {
	got := timevalMilliseconds(syscall.Timeval{Sec: 2, Usec: 345678})
	want := int64(2345)
	if got != want {
		t.Fatalf("timevalMilliseconds() = %d, want %d", got, want)
	}
}
