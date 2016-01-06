package metrics

import "time"

type Stopwatch interface {
	Stop()
}

type stopwatch struct {
	name      string
	startTime time.Time
	receiver  Receiver
}

func (stopwatch *stopwatch) Stop() {
	latencyMicros := time.Now().Sub(stopwatch.startTime) / time.Microsecond
	stopwatch.receiver.AddStat(stopwatch.name+"_us", float64(latencyMicros))
}
