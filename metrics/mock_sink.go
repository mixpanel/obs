package metrics

import (
	"fmt"
	"strings"
	"sync"
)

type MockSink struct {
	mutex        sync.Mutex
	numFlushes   int
	ignorePrefix string
	invocations  map[string]int
}

func (sink *MockSink) invocationKey(metric string, tags Tags, value float64, mtype metricType) string {
	return fmt.Sprintf("%v, %v, %v, %v\n", metric, tags, value, mtype)
}

func (sink *MockSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	if len(sink.ignorePrefix) > 0 && strings.HasPrefix(metric, sink.ignorePrefix) {
		return nil
	}

	formatted := sink.invocationKey(metric, tags, value, metricType)
	sink.invocations[formatted]++
	return nil
}

func (sink *MockSink) Flush() error {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	sink.numFlushes++
	return nil
}

func (sink *MockSink) Close() {}

func (sink *MockSink) NumFlushes() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return sink.numFlushes
}

func (sink *MockSink) NumInvocations() int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return len(sink.invocations)
}

func (sink *MockSink) GetInvocation(metric string, tags Tags, value float64, mtype string) int {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return sink.invocations[sink.invocationKey(metric, tags, value, metricType(mtype))]
}

func NewMockSink(ignorePrefix string) *MockSink {
	return &MockSink{
		numFlushes:   0,
		ignorePrefix: ignorePrefix,
		invocations:  make(map[string]int),
	}
}
