package metrics

import (
	"math/rand"
	"testing"
	"time"
)

func getSample(startWindow, maxWindow int, duration string) *TimeWindowSample {
	d, _ := time.ParseDuration(duration)
	return &TimeWindowSample{
		maxWindowSize: maxWindow,
		timeWindow:    d,
		scaleFactor:   1.5,
		values:        make([]timedValue, startWindow),
	}
}

func newArray(start int, n int) []int64 {
	ret := make([]int64, n)
	for i := 0; i < n; i++ {
		ret[i] = int64(start + i)
	}
	return ret
}

func compareValues(expected []int64, received []int64, t *testing.T) {
	for i := 0; i < len(expected); i++ {
		if received[i] != expected[i] {
			t.Errorf("expected %d, received %d", expected[i], received[i])
		}
	}
}

func TestTimeWindowSampleUpdate(t *testing.T) {
	sample := getSample(4096, 4096, "60s")
	expected := newArray(0, 100)
	for i := 0; i < 100; i++ {
		sample.Update(int64(i))
	}
	compareValues(expected, sample.Values(), t)
}

func TestTimeWindowSampleSmallWindow(t *testing.T) {
	sample := getSample(50, 50, "60s")
	for i := 0; i < 100; i++ {
		sample.Update(int64(i))
	}
	compareValues(newArray(50, 50), sample.Values(), t)
}

func TestTimeWindowSampleScaleUp(t *testing.T) {
	sample := getSample(50, 100, "60s")
	for i := 0; i < 200; i++ {
		sample.Update(int64(i))
	}

	if len(sample.values) > 100 {
		t.Error("sample values not scaled up correctly")
	}
	compareValues(newArray(100, 100), sample.Values(), t)
}

func TestTimeWindowSampleDropped(t *testing.T) {
	sample := getSample(50, 50, "60s")
	for i := 0; i < 100; i++ {
		sample.Update(int64(i))
	}
	compareValues(newArray(50, 50), sample.Values(), t)
	if sample.Dropped() != int64(50) {
		t.Error("did not drop values")
	}
}

func TestTimeWindowSampleScaleDown(t *testing.T) {
	sample := getSample(50, 100, "60s")
	sample.Update(int64(0))

	if len(sample.values) >= 50 {
		t.Error("sample values not scaled down")
	}
	compareValues(newArray(0, 1), sample.Values(), t)
}

func TestTimeWindowSampleTime(t *testing.T) {
	sample := getSample(50, 100, "100ms")
	for i := 0; i < 10; i++ {
		sample.Update(int64(i))
	}
	compareValues(newArray(0, 10), sample.Values(), t)
	time.Sleep(200 * time.Millisecond)
	compareValues(newArray(0, 0), sample.Values(), t)
}

func TestTimeWindowSampleTimeUpdate(t *testing.T) {
	sample := getSample(50, 100, "100ms")
	for i := 0; i < 10; i++ {
		sample.Update(int64(i))
	}
	time.Sleep(200 * time.Millisecond)
	for i := 10; i < 100; i++ {
		sample.Update(int64(i))
	}
	compareValues(newArray(10, 90), sample.Values(), t)
}

func TestTimeWindowSampleMultipleRounds(t *testing.T) {
	sample := getSample(50, 1000, "1s")
	for i := 0; i < 100000; i++ {
		sample.Update(int64(i))
	}
	compareValues(newArray(100000-1000, 1000), sample.Values(), t)
}

func BenchmarkTimeWindowSample(b *testing.B) {
	rand.Seed(31)
	sample := getSample(4096, 8192, "5m")
	for i := 0; i < b.N; i++ {
		sample.Update(rand.Int63())
	}
}

func BenchmarkTimeWindowSamplePercentile(b *testing.B) {
	rand.Seed(31)
	sample := getSample(4096, 8192, "5m")
	for i := 0; i < 100000; i++ {
		sample.Update(rand.Int63())
	}

	percentiles := []float64{0.5, 0.9, 0.99}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sample.Percentile(percentiles[i%len(percentiles)])
	}
}
