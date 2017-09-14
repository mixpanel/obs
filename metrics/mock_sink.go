package metrics

import (
	"fmt"
	"sync"
)

// MockSink is the mock implementation of sink
// to be used in tests for mocking purposes
type MockSink struct {
	mutex       sync.Mutex
	numFlushes  int
	Invocations map[string]int
}

// Handle simluates piping out the metrics with tags and a value
// increses counters that can be asserted
func (sink *MockSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	formatted := fmt.Sprintf("%v, %v, %v, %v\n", metric, tags, value, metricType)
	sink.Invocations[formatted]++
	return nil
}

// Flush simulates the flush of the buffered
// metrics
func (sink *MockSink) Flush() error {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	sink.numFlushes--
	return nil
}

// Close simulates close of the flush loop
func (sink *MockSink) Close() {}

// NumFlushes returns number of times
// flush was called
func (sink *MockSink) NumFlushes() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return sink.numFlushes
}

// NumInvocations returns the number of invocations
// to the handle
func (sink *MockSink) NumInvocations() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return len(sink.Invocations)
}

// NewMockSink returns the mock sink that
// adheres to the Sink interface and
// has utility methods to assert on
func NewMockSink() *MockSink {
	return &MockSink{
		numFlushes:  1,
		Invocations: make(map[string]int),
	}
}
