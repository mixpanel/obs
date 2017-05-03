package metrics

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkLocalSinkCounters(b *testing.B) {
	sink := NewLocalSink(NullSink, 1e18)
	for i := 0; i < b.N; i++ {
		sink.Handle(fmt.Sprintf("metric_%d", rand.Intn(10)), Tags{
			"a": "A",
			"b": "B",
		}, rand.Float64(), "ct")
	}
}

func BenchmarkLocalSinkStats(b *testing.B) {
	sink := NewLocalSink(NullSink, 1e18)
	for i := 0; i < b.N; i++ {
		sink.Handle(fmt.Sprintf("metric_%d", rand.Intn(10)), Tags{
			"a": "A",
			"b": "B",
		}, rand.Float64(), "h")
	}
}

func BenchmarkLocalSinkFlush(b *testing.B) {
	sink := NewLocalSink(NullSink, 1e18)

	for i := 0; i < 1000; i++ {
		sink.Handle(fmt.Sprintf("counter_%d", i), nil, 1.0, "ct")
		sink.Handle(fmt.Sprintf("stat_%d", i), nil, rand.Float64(), "h")
	}

	for i := 0; i < b.N; i++ {
		sink.Flush()
	}
}

func TestLocalSinkCounter(t *testing.T) {
	local, test := newLocalTestSink()
	local.Handle("test", nil, 1, metricTypeCounter)
	local.Handle("test", nil, 1, metricTypeCounter)
	local.Flush()

	assert.Equal(t, 1, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 2, metricTypeGauge), test.stats[0])
}

func TestLocalSinkCounterWithFlushThreshold(t *testing.T) {
	test := &testSink{}
	local := NewLocalSink(test, 1)

	local.Handle("test", nil, 1, metricTypeCounter)

	local.Flush()

	// The second flush should be a no-op because the value hasn't changed
	assert.Equal(t, 1, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 1, metricTypeGauge), test.stats[0])

	local.Flush()
	// The second flush should be a no-op because the value hasn't changed
	assert.Equal(t, 1, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 1, metricTypeGauge), test.stats[0])

	local.Handle("test", nil, 1, metricTypeCounter)
	local.Flush()

	assert.Equal(t, 2, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 2, metricTypeGauge), test.stats[1])
}

func TestLocalSinkCounterWithTags(t *testing.T) {
	local, test := newLocalTestSink()
	local.Handle("test", Tags{"a": "b"}, 1, metricTypeCounter)
	local.Handle("test", Tags{"a": "b"}, 1, metricTypeCounter)
	local.Handle("test", Tags{"a": "c"}, 1, metricTypeCounter)
	local.Flush()

	mp := map[string]bool{
		formatMetric("test", Tags{"a": "b"}, 2, metricTypeGauge): true,
		formatMetric("test", Tags{"a": "c"}, 1, metricTypeGauge): true,
	}

	assert.Equal(t, 2, len(test.stats))
	assert.Equal(t, true, mp[test.stats[0]])
	assert.Equal(t, true, mp[test.stats[1]])
	assert.NotEqual(t, test.stats[0], test.stats[1])
}

func TestLocalSinkGauge(t *testing.T) {
	local, test := newLocalTestSink()
	local.Handle("test", nil, 1, metricTypeGauge)
	local.Handle("test", nil, 2, metricTypeGauge)
	local.Flush()

	assert.Equal(t, 1, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 2, metricTypeGauge), test.stats[0])

	local.Handle("test", nil, 3, metricTypeGauge)
	local.Flush()

	assert.Equal(t, 2, len(test.stats))
	assert.Equal(t, formatMetric("test", nil, 3, metricTypeGauge), test.stats[1])
}

func TestLocalSinkGaugeWithTags(t *testing.T) {
	local, test := newLocalTestSink()
	local.Handle("test", Tags{"a": "b"}, 1, metricTypeGauge)
	local.Handle("test", Tags{"a": "c"}, 2, metricTypeGauge)
	local.Flush()

	mp := map[string]bool{
		formatMetric("test", Tags{"a": "b"}, 1, metricTypeGauge): true,
		formatMetric("test", Tags{"a": "c"}, 2, metricTypeGauge): true,
	}

	assert.Equal(t, 2, len(test.stats))
	assert.Equal(t, true, mp[test.stats[0]])
	assert.Equal(t, true, mp[test.stats[1]])
	assert.NotEqual(t, test.stats[0], test.stats[1])
}

func unpackFlushed(s string) (string, float64, string) {
	var v float64
	split := strings.Split(s, " ")
	fmt.Sscanf(split[2], "%g", &v)
	return split[0], v, split[3]
}

func TestLocalSinkStat(t *testing.T) {
	local, test := newLocalTestSink()
	for i := 1; i <= 100; i++ {
		local.Handle("test", nil, float64(i), metricTypeStat)
	}
	local.Flush()

	mp := map[string]bool{
		formatMetric("test.count", nil, 100, metricTypeGauge):       true,
		formatMetric("test.avg", nil, 50, metricTypeGauge):          true,
		formatMetric("test.max", nil, 100, metricTypeGauge):         true,
		formatMetric("test.min", nil, 1, metricTypeGauge):           true,
		formatMetric("test.median", nil, 50, metricTypeGauge):       true,
		formatMetric("test.90percentile", nil, 90, metricTypeGauge): true,
		formatMetric("test.99percentile", nil, 99, metricTypeGauge): true,
		formatMetric("test._dropped", nil, 0, metricTypeGauge):      true,
	}

	mpStats := make(map[string]bool)

	for _, s := range test.stats {
		name, value, mt := unpackFlushed(s)
		mpStats[formatMetric(name, nil, float64(math.Floor(value)), metricType(mt))] = true
	}

	assert.Equal(t, mp, mpStats)
}

func TestLocalSinkStatWithTags(t *testing.T) {
	local, test := newLocalTestSink()
	for i := 1; i <= 100; i++ {
		local.Handle("test", Tags{"a": "b"}, float64(i), metricTypeStat)
	}
	local.Flush()

	mp := map[string]bool{
		formatMetric("test.count", Tags{"a": "b"}, 100, metricTypeGauge):       true,
		formatMetric("test.avg", Tags{"a": "b"}, 50, metricTypeGauge):          true,
		formatMetric("test.max", Tags{"a": "b"}, 100, metricTypeGauge):         true,
		formatMetric("test.min", Tags{"a": "b"}, 1, metricTypeGauge):           true,
		formatMetric("test.median", Tags{"a": "b"}, 50, metricTypeGauge):       true,
		formatMetric("test.90percentile", Tags{"a": "b"}, 90, metricTypeGauge): true,
		formatMetric("test.99percentile", Tags{"a": "b"}, 99, metricTypeGauge): true,
		formatMetric("test._dropped", Tags{"a": "b"}, 0, metricTypeGauge):      true,
	}

	mpStats := make(map[string]bool)

	for _, s := range test.stats {
		name, value, mt := unpackFlushed(s)
		mpStats[formatMetric(name, Tags{"a": "b"}, float64(math.Floor(value)), metricType(mt))] = true
	}

	assert.Equal(t, mp, mpStats)
}

func TestLocalSinkCumulativeFrequency(t *testing.T) {
	local, test := newLocalTestSink()
	for i := 100000; i <= 100000000; i *= 2 {
		// Cumulative frequency counters should be created for this metric.
		local.Handle("arb.distributed_query_server.latency_us", Tags{"a": "b"}, float64(i), metricTypeStat)
		// No cumulative frequency counters should be created for this metrics.
		local.Handle("foo", Tags{"a": "b"}, float64(i), metricTypeStat)
	}
	local.Flush()

	metric := "arb.distributed_query_server.latency_us"
	expected := map[string]bool{
		formatMetric(metric + ".less_than.250000", Tags{"a": "b"}, 2, metricTypeGauge):   true,
		formatMetric(metric + ".less_than.500000", Tags{"a": "b"}, 3, metricTypeGauge):   true,
		formatMetric(metric + ".less_than.1000000", Tags{"a": "b"}, 4, metricTypeGauge):  true,
		formatMetric(metric + ".less_than.5000000", Tags{"a": "b"}, 6, metricTypeGauge):  true,
		formatMetric(metric + ".less_than.15000000", Tags{"a": "b"}, 8, metricTypeGauge): true,
		formatMetric(metric + ".less_than.30000000", Tags{"a": "b"}, 9, metricTypeGauge): true,
		formatMetric(metric + ".less_than.60000000", Tags{"a": "b"}, 10, metricTypeGauge): true,
		formatMetric(metric + ".less_than.inf", Tags{"a": "b"}, 10, metricTypeGauge): true,
	}

	notExpected := map[string]bool{
		formatMetric("foo.less_than.250000", Tags{"a": "b"}, 2, metricTypeGauge):   true,
		formatMetric("foo.less_than.500000", Tags{"a": "b"}, 3, metricTypeGauge):   true,
		formatMetric("foo.less_than.1000000", Tags{"a": "b"}, 4, metricTypeGauge):  true,
		formatMetric("foo.less_than.5000000", Tags{"a": "b"}, 6, metricTypeGauge):  true,
		formatMetric("foo.less_than.15000000", Tags{"a": "b"}, 8, metricTypeGauge): true,
		formatMetric("foo.less_than.30000000", Tags{"a": "b"}, 9, metricTypeGauge): true,
		formatMetric("foo.less_than.60000000", Tags{"a": "b"}, 10, metricTypeGauge): true,
		formatMetric("foo.less_than.inf", Tags{"a": "b"}, 10, metricTypeGauge): true,
	}

	actual := make(map[string]bool)

	for _, s := range test.stats {
		name, value, mt := unpackFlushed(s)
		actual[formatMetric(name, Tags{"a": "b"}, float64(math.Floor(value)), metricType(mt))] = true
	}

	for key := range expected {
		assert.Contains(t, actual, key)
	}

	for key := range notExpected {
		assert.NotContains(t, actual, key)
	}
}

func newLocalTestSink() (Sink, *testSink) {
	dst := &testSink{}
	local := NewLocalSink(dst, 1e18)
	return local, dst
}

type testSink struct {
	stats []string
}

func formatMetric(metric string, tags Tags, value float64, metricType metricType) string {
	return fmt.Sprintf("%s %s %g %s", metric, FormatTags(tags), value, metricType)
}

func (sink *testSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	sink.stats = append(sink.stats, formatMetric(metric, tags, value, metricType))
	return nil
}

func (sink *testSink) Flush() error { return nil }
func (sink *testSink) Close()       {}
