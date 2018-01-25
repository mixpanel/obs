package obs

import (
	"runtime"
	"time"

	"github.com/mixpanel/obs/metrics"
)

func reportGCMetrics(interval time.Duration, done <-chan struct{}, r metrics.Receiver) {
	r = r.ScopePrefix("gc")
	numGCs := uint32(0)

	memstats := &runtime.MemStats{}
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-time.After(interval):
				numGCs = reportGCsSince(memstats, numGCs, r)
			}
		}
	}()
}

func reportGCsSince(memstats *runtime.MemStats, lastCount uint32, r metrics.Receiver) uint32 {
	runtime.ReadMemStats(memstats)
	newCount := memstats.NumGC
	expected := newCount - lastCount
	if expected == 0 {
		return newCount
	}

	r.IncrBy("cycles", float64(expected))
	numPresent := uint32(len(memstats.PauseNs))
	if numPresent < expected {
		r.IncrBy("cycles_missed", float64(expected-numPresent))
		lastCount = newCount - numPresent - 1
	}

	for i := lastCount + 1; i <= newCount; i++ {
		index := (i + numPresent - 1) % numPresent
		pauseNs := memstats.PauseNs[index]
		r.AddStat("pause_ns", float64(pauseNs))
	}
	r.SetGauge("heap_allocated_bytes", float64(memstats.HeapAlloc))
	r.SetGauge("total_heap_allocated_bytes", float64(memstats.TotalAlloc))
	r.SetGauge("system_allocated_bytes", float64(memstats.Sys))
	return newCount
}
