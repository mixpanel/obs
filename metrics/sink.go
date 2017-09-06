package metrics

// Sink is the interface to where the metrics
// get reported. Sink is the actual output pipe
// of the metrics reporting.
// An example of sink is statsd sink
type Sink interface {
	Handle(metric string, tags Tags, value float64, metricType metricType) error
	Flush() error
	Close()
}

type nullSink struct{}

func (sink *nullSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	return nil
}

func (sink *nullSink) Flush() error {
	return nil
}

func (sink *nullSink) Close() {
}

// NullSink is the no op sink
var NullSink Sink = &nullSink{}
