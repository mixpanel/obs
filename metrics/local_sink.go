package metrics

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_metrics "github.com/rcrowley/go-metrics"
)

type metricKey struct {
	metricType metricType
	name       string
}

type localSink struct {
	counters _metrics.Registry
	gauges   _metrics.Registry
	stats    _metrics.Registry
	dst      Sink

	flushThreshold int64

	registerLock sync.Mutex
	currentGen   int64
	touched      map[metricKey]int64

	flushLock sync.Mutex
}

func (sink *localSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	if len(metric) == 0 {
		return errors.New("cannot handle empty metric")
	}

	formatted := metric + "|" + FormatTags(tags)

	key := metricKey{metricType: metricType, name: formatted}

	sink.registerLock.Lock()
	defer sink.registerLock.Unlock()
	sink.touched[key] = sink.currentGen

	switch metricType {
	case metricTypeCounter:
		counter := sink.counters.Get(formatted)
		if counter == nil {
			counter = _metrics.NewCounter()
			defer sink.counters.Register(formatted, counter)
		}
		counter.(_metrics.Counter).Inc(int64(value))
	case metricTypeGauge:
		gauge := sink.gauges.Get(formatted)
		if gauge == nil {
			gauge = _metrics.NewGaugeFloat64()
			// N.B. defer so that we only register after we've set the value.
			defer sink.gauges.Register(formatted, gauge)
		}
		gauge.(_metrics.GaugeFloat64).Update(value)
	case metricTypeStat:
		stat := sink.stats.Get(formatted)
		if stat == nil {
			sample := _metrics.NewTimeWindowSample(4096, 8192, 300*time.Second)
			stat = _metrics.NewHistogram(sample)
			// N.B. defer so that we only register after we've set the value.
			defer sink.stats.Register(formatted, stat)
		}
		stat.(_metrics.Histogram).Update(int64(value))
	default:
		return errors.New(fmt.Sprintf("unknown metric type: %s", metricType))
	}
	return nil
}

func (sink *localSink) Flush() error {
	sink.registerLock.Lock()
	toFlush := make(map[metricKey]int64, len(sink.touched))
	gen := sink.currentGen
	cutoff := gen - sink.flushThreshold
	for k, v := range sink.touched {
		if v > cutoff {
			toFlush[k] = v
		} else {
			delete(sink.touched, k)
		}
	}
	sink.currentGen++
	sink.registerLock.Unlock()

	shouldFlush := func(mt metricType, name string) bool {
		_, ok := toFlush[metricKey{mt, name}]
		return ok
	}
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
			if shouldFlush(metricTypeCounter, name) {
				sink.dst.Handle(metricName, tags, float64(metric.Count()), metricTypeGauge)
			}
		case _metrics.GaugeFloat64:
			if shouldFlush(metricTypeGauge, name) {
				sink.dst.Handle(metricName, tags, float64(metric.Value()), metricTypeGauge)
			}
		case _metrics.Histogram:
			h := metric.Snapshot()
			p := h.Percentiles([]float64{0.5000, 0.9000, 0.9900})
			if shouldFlush(metricTypeStat, name) {
				sink.dst.Handle(metricName+".count", tags, float64(h.Count()), metricTypeGauge)
				sink.dst.Handle(metricName+".max", tags, float64(h.Max()), metricTypeGauge)
				sink.dst.Handle(metricName+".min", tags, float64(h.Min()), metricTypeGauge)
				sink.dst.Handle(metricName+".median", tags, p[0], metricTypeGauge)
				sink.dst.Handle(metricName+".avg", tags, h.Mean(), metricTypeGauge)
				sink.dst.Handle(metricName+".90percentile", tags, p[1], metricTypeGauge)
				sink.dst.Handle(metricName+".99percentile", tags, p[2], metricTypeGauge)
				sink.dst.Handle(metricName+"._dropped", tags, float64(h.Dropped()), metricTypeGauge)
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

		flushThreshold: int64(flushThreshold),

		touched: make(map[metricKey]int64),
	}
}
