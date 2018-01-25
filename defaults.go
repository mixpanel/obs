package obs

import (
	"fmt"
	"syscall"
	"time"

	"github.com/mixpanel/obs/closesig"
	"github.com/mixpanel/obs/logging"
	"github.com/mixpanel/obs/metrics"
	"github.com/mixpanel/obs/tracing"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"

	"context"
)

type Options struct {
	LogLevel string `long:"obs.log-level" description:"NEVER, DEBUG, INFO, WARN, ERROR or CRITICAL" default:"INFO"`
}

type Closer func()

type Option func(*obsOptions)

// SampleRate takes in an int n, and sets the sampling rate of traces to be 1 / n
func SampleRate(n uint64) Option {
	return func(o *obsOptions) {
		o.tracerOpts.ShouldSample = func(traceID uint64) bool { return traceID%n == 0 }
	}
}

var NoTraces Option = func(o *obsOptions) {
	o.tracerOpts.ShouldSample = func(traceID uint64) bool { return false }
}

type obsOptions struct {
	tracerOpts basictracer.Options
}

// TODO(shimin): InitGCP should be able to set default tags (project, cluster, host) from metadata service.
// It should also allow the caller to pass in other tags.
func InitGCP(ctx context.Context, serviceName, logLevel string, opts ...Option) (FlightRecorder, Closer) {
	sig := closesig.Client(closesig.DefaultPort)
	l := logging.New("NEVER", logLevel, "", "json")

	obsOpts := obsOptions{tracerOpts: basictracer.DefaultOptions()}
	SampleRate(100)(&obsOpts)
	for _, o := range opts {
		o(&obsOpts)
	}

	tracer, closeTracer := tracing.New(obsOpts.tracerOpts)
	fr, closer := initFR(ctx, serviceName, l, tracer)
	return fr, func() {
		closeTracer()
		closer()
		sig()
	}
}

func InitCli(ctx context.Context, name, logLevel string) (FlightRecorder, Closer) {
	fr := NewFlightRecorder(name, metrics.Null, logging.New(logLevel, logLevel, "", "text"),
		opentracing.NoopTracer{})
	return fr, func() {}
}

func initFR(ctx context.Context, serviceName string, l logging.Logger, tr opentracing.Tracer) (FlightRecorder, Closer) {
	sink, err := metrics.NewStatsdSink("127.0.0.1:8125")
	if err != nil {
		l.Critical("error initializing metrics", logging.Fields{}.WithError(err))
		panic(fmt.Errorf("error initializing metrics: %v", err))
	}

	mr := metrics.NewReceiver(sink).ScopePrefix(serviceName)
	l = l.Named(serviceName)
	Metrics = mr
	Log = l

	done := make(chan struct{})
	reportStandardMetrics(mr, done)

	fr := NewFlightRecorder(serviceName, mr, l, tr)
	// TODO: make this work. currently obs.logging uses SetOutput on the global logging which makes this a circlular dependency
	// log.SetOutput(stderrAdapter{fr.WithSpan(ctx)})

	return fr, func() {
		close(done)
		sink.Close()
	}
}

func reportStandardMetrics(mr metrics.Receiver, done <-chan struct{}) {
	reportGCMetrics(3*time.Second, done, mr)
	reportVersion(done, mr)
	reportUptime(done, mr)
	reportRusage(done, mr)
}

func reportVersion(done <-chan struct{}, receiver metrics.Receiver) {
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				// TODO: Add back
				//receiver.SetGauge("git_version", float64(version.Int()))
				next = time.After(60 * time.Second)
			}
		}
	}()
}

func reportUptime(done <-chan struct{}, receiver metrics.Receiver) {
	startTime := time.Now()
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				uptime := time.Now().Sub(startTime)
				receiver.SetGauge("uptime_sec", uptime.Seconds())
				next = time.After(60 * time.Second)
			}
		}
	}()
}

func reportRusage(done <-chan struct{}, receiver metrics.Receiver) {
	receiver = receiver.ScopePrefix("rusage")
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				var rusage syscall.Rusage
				err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage)
				if err == nil {
					receiver.SetGauge("user_us", float64(rusage.Utime.Sec*1e6+int64(rusage.Utime.Usec)))
					receiver.SetGauge("system_us", float64(rusage.Stime.Sec*1e6+int64(rusage.Stime.Usec)))
					receiver.SetGauge("voluntary_cs", float64(rusage.Nvcsw))
					receiver.SetGauge("involuntary_cs", float64(rusage.Nivcsw))
				}
				next = time.After(60 * time.Second)
			}
		}
	}()
}

type stderrAdapter struct {
	fs FlightSpan
}

func (sa stderrAdapter) Write(bs []byte) (int, error) {
	sa.fs.Info(string(bs), Vals(getCallerContext(1)))
	return len(bs), nil
}
