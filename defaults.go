package obs

import (
	"fmt"
	"obs/logging"
	"obs/metrics"
	"time"
	"version"

	"github.com/jessevdk/go-flags"
)

var Log logging.Logger = logging.Null
var Sink metrics.Sink = metrics.NullSink
var Metrics metrics.Receiver = metrics.Null

type ObsOptions struct {
	SyslogLevel     string `long:"syslog.level" default:"NEVER" description:"One of CRIT, ERR, WARN, INFO, DEBUG, NEVER"`
	LogLevel        string `long:"log.level" default:"INFO" description:"One of CRIT, ERR, WARN, INFO, DEBUG, NEVER"`
	LogPath         string `long:"log.path" description:"File path to log. uses stderr if not set"`
	LogFormat       string `long:"log.format" description:"Format of log output" default:"text" choice:"text" choice:"json"`
	MetricsEndpoint string `long:"metrics-endpoint" description:"Address (host:port) to send metrics"`
}

func NewOptions(parser *flags.Parser) *ObsOptions {
	options := &ObsOptions{}
	group, err := parser.AddGroup("Observability", "", options)
	if err != nil {
		panic(err)
	}
	group.Namespace = "obs"
	return options
}

func (opts *ObsOptions) Init(metricsPrefix string) {
	opts.InitLogging()
	if sink, err := metrics.NewStatsdSink(opts.MetricsEndpoint); err != nil {
		panic(fmt.Errorf("error initializing metrics: {{error_message}}", logging.Fields{}.WithError(err)))
	} else {
		opts.InitWithSink(metricsPrefix, sink)
	}
}

func (opts *ObsOptions) InitLogging() {
	Log = logging.New(opts.SyslogLevel, opts.LogLevel, opts.LogPath, opts.LogFormat)
}

// InitLogging should already have been invoked
func (opts *ObsOptions) InitWithSink(metricsPrefix string, sink metrics.Sink) {
	Sink = sink
	receiver := metrics.NewReceiver(Sink)
	Metrics = receiver.ScopePrefix(metricsPrefix)
	ReportGCMetrics(3*time.Second, Metrics)
	ReportVersion(Metrics)
	ReportUptime(Metrics)
}

func ReportVersion(receiver metrics.Receiver) {
	go func() {
		for {
			receiver.SetGauge("git_version", float64(version.Int()))
			time.Sleep(60 * time.Second)
		}
	}()
}

func ReportUptime(receiver metrics.Receiver) {
	startTime := time.Now()
	go func() {
		for {
			uptime := time.Now().Sub(startTime)
			receiver.SetGauge("uptime_sec", uptime.Seconds())
			time.Sleep(60 * time.Second)
		}
	}()
}
