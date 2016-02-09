package metrics

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	_metrics "github.com/rcrowley/go-metrics"
)

type value struct {
	value        int64
	unchangedFor int
}

type localSink struct {
	counters _metrics.Registry
	gauges   _metrics.Registry
	stats    _metrics.Registry
	dst      Sink

	flushLock      sync.Mutex
	flushThreshold int
	flushedValues  map[string]*value
}

func (sink *localSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	if len(metric) == 0 {
		return errors.New("cannot handle empty metric")
	}

	formatted := metric + "|" + FormatTags(tags)

	switch metricType {
	case metricTypeCounter:
		counter := _metrics.GetOrRegisterCounter(formatted, sink.counters)
		counter.Inc(int64(value))
	case metricTypeGauge:
		gauge := _metrics.GetOrRegisterGaugeFloat64(formatted, sink.gauges)
		gauge.Update(value)
	case metricTypeStat:
		// TODO: Create a windowed histogram.
		// alpha value copied from go-metrics exmaples
		stat := sink.stats.Get(formatted)
		if stat == nil {
			sample := _metrics.NewExpDecaySample(4096, 0.015)
			stat = _metrics.GetOrRegisterHistogram(formatted, sink.stats, sample)
		}
		stat.(_metrics.Histogram).Update(int64(value))
	default:
		return errors.New(fmt.Sprintf("unknown metric type: %s", metricType))
	}
	return nil
}

// The current implementation will not flush a counter if it was updated but the value remained the same (possibly because of an incr(0) or
// incr(-X) followed by incr(+X).
// TODO: Fix this if it starts becoming a problem.
// NOTE: Caller should acquire sink.flushLock before invoking this function.
func (sink *localSink) shouldFlush(name string, v int64) bool {
	oldValue, ok := sink.flushedValues[name]
	if !ok || oldValue.value != v {
		sink.flushedValues[name] = &value{v, 1}
		return true
	}

	oldValue.unchangedFor++
	return oldValue.unchangedFor <= sink.flushThreshold
}

func (sink *localSink) Flush() error {
	sink.flushLock.Lock()
	defer sink.flushLock.Unlock()

	flush := func(name string, i interface{}) {
		split := strings.Split(name, "|")
		if len(split) != 2 {
			log.Printf("unparseable metric: %s", name)
			return
		}

		metricName := strings.TrimSpace(split[0])

		tags, err := ParseTags(strings.TrimSpace(split[1]))
		if err != nil {
			log.Printf("could not parse tags: %s", split[1])
			return
		}

		switch metric := i.(type) {
		case _metrics.Counter:
			if sink.shouldFlush(name, metric.Count()) {
				sink.dst.Handle(metricName, tags, float64(metric.Count()), metricTypeGauge)
			}
		case _metrics.GaugeFloat64:
			sink.dst.Handle(metricName, tags, float64(metric.Value()), metricTypeGauge)
		case _metrics.Histogram:
			h := metric.Snapshot()
			p := h.Percentiles([]float64{0.5000, 0.9000, 0.9900})
			if sink.shouldFlush(name+".count", h.Count()) {
				sink.dst.Handle(metricName+".count", tags, float64(h.Count()), metricTypeGauge)
				sink.dst.Handle(metricName+".max", tags, float64(h.Max()), metricTypeGauge)
				sink.dst.Handle(metricName+".median", tags, p[0], metricTypeGauge)
				sink.dst.Handle(metricName+".avg", tags, h.Mean(), metricTypeGauge)
				sink.dst.Handle(metricName+".90percentile", tags, p[1], metricTypeGauge)
				sink.dst.Handle(metricName+".99percentile", tags, p[2], metricTypeGauge)
			}
		default:
			// Ignore all other metrics
		}
	}

	sink.counters.Each(flush)
	sink.gauges.Each(flush)
	sink.stats.Each(flush)

	sink.dst.Flush()
	return nil
}

func (sink *localSink) Close() {
	sink.Flush()
	sink.counters.UnregisterAll()
	sink.gauges.UnregisterAll()
	sink.stats.UnregisterAll()
}

func NewLocalSink(dst Sink, flushThreshold int) Sink {
	return &localSink{
		counters: _metrics.NewRegistry(),
		gauges:   _metrics.NewRegistry(),
		stats:    _metrics.NewRegistry(),
		dst:      dst,

		flushThreshold: flushThreshold,
		flushedValues:  make(map[string]*value),
	}
}
