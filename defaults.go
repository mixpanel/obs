package obs

import (
	"obs/logging"
	"obs/metrics"
	"time"
	"version"
)

var Log logging.Logger
var Metrics metrics.Receiver

func init() {
	Log = logging.NewDefault()
	metricsReceiver, err := metrics.NewDefault()
	if err != nil {
		Log.Errorf("error initializing metrics: {{error_message}}", logging.Fields{}.WithError(err))
	} else {
		Metrics = metricsReceiver
	}
}

func ReportVersion(receiver metrics.Receiver) {
	go func() {
		for {
			receiver.SetGauge("git_version", float64(version.Int()))
			time.Sleep(60 * time.Second)
		}
	}()
}
