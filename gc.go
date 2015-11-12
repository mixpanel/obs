package obs

import (
	"io"
	"obs/metrics"
	"runtime"
	"time"
)

type closerChan chan struct{}

func (c closerChan) Close() error {
	close(c)
	return nil
}

func ReportGCMetrics(interval time.Duration, r metrics.Receiver) io.Closer {
	r = r.ScopePrefix("gc")
	numGCs := uint32(0)

	memstats := &runtime.MemStats{}
	closed := make(chan struct{})
	go func() {
		for {
			select {
			case _ = <-closed:
				return
			case _ = <-time.After(interval):
				numGCs = reportGCsSince(memstats, numGCs, r)
			}
		}
	}()

	return closerChan(closed)
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
	r.SetGauge("system_allocated_bytes", float64(memstats.Sys))
	return newCount
}
