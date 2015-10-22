package topk

import (
	"fmt"
	"obs/metrics"
	"sync"
)

const (
	chanBufferSize = bufferSize * 2
)

// A best-effort receiver. We buffer up to chanBufferSize values, and process them in a single goroutine.
// The top K values will be reported to the provided MetricsReceiver.
type Receiver interface {
	Track(int32)
	Close()
}

type receiver struct {
	ch      chan<- int32
	metrics metrics.MetricsReceiver
	wg      *sync.WaitGroup
}

func New(metrics metrics.MetricsReceiver, k int) Receiver {
	ch := make(chan int32, chanBufferSize)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		t := newTracker(k)
		for value := range ch {
			if !t.track(value) {
				metrics.Incr("sampled")
			}
			if t.isTopK(value) {
				metrics.Incr(fmt.Sprintf("top.%d", value))
			}
		}
		wg.Done()
	}()
	return &receiver{ch, metrics, wg}
}

func (t *receiver) Close() {
	close(t.ch)
	t.wg.Wait()
}

func (t *receiver) Track(value int32) {
	select {
	case t.ch <- value:
	default:
		t.metrics.Incr("dropped")
	}
}
