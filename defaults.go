package obs

import (
	"obs/logging"
	"obs/metrics"
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
