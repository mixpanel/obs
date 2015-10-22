package topk

import (
	"obs/metrics"
	"testing"

	"github.com/stretchr/testify/mock"
)

func TestReceiver(t *testing.T) {
	mockMetrics := newMockMetrics()
	receiver := New(mockMetrics, 4)

	mockMetrics.On("Incr", mock.AnythingOfType("string")).Return()
	for i := 0; i < bufferSize*2; i++ {
		receiver.Track(1 + int32(i%4))
	}
	receiver.Track(5)
	receiver.Close()

	mockMetrics.AssertCalled(t, "Incr", "top.1")
	mockMetrics.AssertCalled(t, "Incr", "top.2")
	mockMetrics.AssertCalled(t, "Incr", "top.3")
	mockMetrics.AssertCalled(t, "Incr", "top.4")
	mockMetrics.AssertNotCalled(t, "Incr", "top.5")

}

type mockMetrics struct {
	mock.Mock
}

func (mock *mockMetrics) Incr(name string) {
	mock.Called(name)
}

func (mock *mockMetrics) IncrBy(name string, amount float64) {
}

func (mock *mockMetrics) AddStat(name string, value float64) {
}

func (mock *mockMetrics) SetGauge(name string, value float64) {
}

func (mock *mockMetrics) ScopePrefix(prefix string) metrics.MetricsReceiver {
	return mock
}

func (mock *mockMetrics) ScopeTags(tags metrics.Tags) metrics.MetricsReceiver {
	return mock
}

func (mock *mockMetrics) Scope(prefix string, tags metrics.Tags) metrics.MetricsReceiver {
	return mock
}

func (mock *mockMetrics) StartStopwatch(name string) metrics.Stopwatch {
	return nil
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{}
}
