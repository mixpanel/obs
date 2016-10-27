package obs

import (
	"fmt"
	"obs/logging"
	"obs/metrics"
	"obs/tracing"
	"path"
	"time"
	"version"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"

	"golang.org/x/net/context"
)

type Closer func()

type Option func(*obsOptions)

// SampleRate takes in an int n, and sets the sampling rate of traces to be 1 / n
func SampleRate(n uint64) Option {
	return func(o *obsOptions) {
		o.tracerOpts.ShouldSample = func(traceID uint64) bool { return traceID%n == 0 }
	}
}

type obsOptions struct {
	tracerOpts basictracer.Options
}

func InitGCP(ctx context.Context, serviceName string, opts ...Option) (FlightRecorder, Closer) {
	l := logging.New("NEVER", "INFO", "", "json")

	obsOpts := obsOptions{tracerOpts: basictracer.DefaultOptions()}
	SampleRate(100)(&obsOpts)
	for _, o := range opts {
		o(&obsOpts)
	}

	return initFR(ctx, serviceName, l, tracing.New(obsOpts.tracerOpts))
}

func InitSoftlayer(ctx context.Context, serviceName string) (FlightRecorder, Closer) {
	l := logging.New("WARN", "INFO", path.Join("/var/log/mixpanel/", serviceName+".log"), "text")
	return initFR(ctx, serviceName, l, opentracing.NoopTracer{})
}

func initFR(ctx context.Context, serviceName string, l logging.Logger, tr opentracing.Tracer) (FlightRecorder, Closer) {
	sink, err := metrics.NewStatsdSink("127.0.0.1:8125")
	if err != nil {
		l.Critical("error initializing metrics", logging.Fields{}.WithError(err))
		panic(fmt.Errorf("error initializing metrics: %v", err))
	}

	mr := metrics.NewReceiver(sink).ScopePrefix(serviceName)
	l = l.Named(serviceName)

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
}

func reportVersion(done <-chan struct{}, receiver metrics.Receiver) {
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				receiver.SetGauge("git_version", float64(version.Int()))
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

type stderrAdapter struct {
	fs FlightSpan
}

func (sa stderrAdapter) Write(bs []byte) (int, error) {
	sa.fs.Info(string(bs), Vals(getCallerContext(1)))
	return len(bs), nil
}
