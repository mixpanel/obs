package metrics

type Sink interface {
	Handle(metric string, tags Tags, value float64, metricType metricType) error
	Flush() error
	Close()
}
