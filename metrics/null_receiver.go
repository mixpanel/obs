package metrics

var NullReceiver MetricsReceiver = nullReceiver(struct{}{})

type nullReceiver struct{}
type nullStopwatch struct{}

func (nl nullReceiver) Incr(name string) {
}

func (nl nullReceiver) IncrBy(name string, amount float64) {
}

func (nl nullReceiver) AddStat(name string, value float64) {
}

func (nl nullReceiver) SetGauge(name string, value float64) {
}

func (nl nullReceiver) ScopePrefix(prefix string) MetricsReceiver {
	return nl
}

func (nl nullReceiver) ScopeTags(tags Tags) MetricsReceiver {
	return nl
}

func (nl nullReceiver) Scope(prefix string, tags Tags) MetricsReceiver {
	return nl
}

func (nl nullReceiver) StartStopwatch(name string) Stopwatch {
	return nullStopwatch(struct{}{})
}

func (ns nullStopwatch) Stop() {
}
