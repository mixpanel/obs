package obs

import (
	"runtime"
	"testing"
)

func BenchmarkMemStats(b *testing.B) {
	memstats := &runtime.MemStats{}

	for i := 0; i < b.N; i++ {
		runtime.ReadMemStats(memstats)
	}
}
