package obs

import (
	"fmt"
	"io/ioutil"
	"obs/closesig"
	"obs/logging"
	"obs/metrics"
	"obs/tracing"
	"os"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
	"version"

	"golang.org/x/crypto/ssh/terminal"

	gce "cloud.google.com/go/compute/metadata"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"

	"context"
)

const (
	defaultStatsdAddr = "127.0.0.1:8125"
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

func WithSidecarStatsd(o *obsOptions) {
	o.statsdAddress = defaultStatsdAddr
}

func WithNodeStatsd(o *obsOptions) {
	o.statsdAddress = os.Getenv("NODE-IP") + ":8125"
}

func WithEnvStatsd(o *obsOptions) {
	o.statsdAddress = defaultStatsdAddr
	if len(os.Getenv("STATSD_ENDPOINT")) > 0 {
		o.statsdAddress = os.Getenv("STATSD_ENDPOINT")
	}
}

var NoStatsd Option = func(o *obsOptions) {
	o.statsdAddress = ""
}

func WithGCPTagging(o *obsOptions) {
	name, err := ioutil.ReadFile("/etc/podinfo/name")
	if err != nil {
		Log.Warn("error retrieving pod's name", logging.Fields{}.WithError(err))
	} else {
		o.tags["pod.name"] = string(name)
	}
	namespace, err := ioutil.ReadFile("/etc/podinfo/namespace")
	if err != nil {
		Log.Warn("error retrieving pod's namespace", logging.Fields{}.WithError(err))
	} else {
		o.tags["pod.namespace"] = string(namespace)
	}
	labels, err := ioutil.ReadFile("/etc/podinfo/labels")
	if err != nil {
		Log.Warn("error retrieving pod's labels", logging.Fields{}.WithError(err))
	} else {
		for _, label := range strings.Split(string(labels), "\n") {
			kv := strings.SplitN(label, "=", 2)
			o.tags["pod."+kv[0]] = strings.Trim(kv[1], `"`)
		}
	}
}

type obsOptions struct {
	tracerOpts    basictracer.Options
	statsdAddress string
	tags          Tags
}

// TODO(shimin): InitGCP should be able to set default tags (project, cluster, host) from metadata service.
// It should also allow the caller to pass in other tags.
func InitGCP(ctx context.Context, serviceName, logLevel string, opts ...Option) (FlightRecorder, Closer) {
	sig := closesig.Client(closesig.DefaultPort)
	l := logging.New("NEVER", logLevel, "", "json")

	obsOpts := obsOptions{
		tracerOpts: basictracer.DefaultOptions(),
		tags:       Tags{},
	}
	// Default to no tracing, can be enabled with options
	NoTraces(&obsOpts)
	WithEnvStatsd(&obsOpts)
	if len(os.Getenv("GCP_TAGGING")) > 0 {
		WithGCPTagging(&obsOpts)
	}

	for _, o := range opts {
		o(&obsOpts)
	}

	var tracer opentracing.Tracer
	var closeTracer func()
	var sink metrics.Sink

	if obsOpts.statsdAddress != "" {
		sink = newStatsdSink(obsOpts.statsdAddress)
		mr := metrics.NewReceiver(sink).ScopePrefix("tracer").ScopeTags(metrics.Tags{"service": serviceName})
		tracer, closeTracer = tracing.New(mr, obsOpts.tracerOpts)
	} else {
		tracer = opentracing.NoopTracer{}
		closeTracer = func() {}
		sink = metrics.NullSink
	}

	fr, closer := initFR(ctx, sink, serviceName, l, tracer)
	fr = fr.ScopeTags(obsOpts.tags)
	return fr, func() {
		closeTracer()
		closer()
		sig()
	}
}

func InitSoftlayer(ctx context.Context, serviceName, logLevel string) (FlightRecorder, Closer) {
	l := logging.New("WARN", logLevel, path.Join("/var/log/mixpanel/", serviceName+".log"), "text")
	sink := newStatsdSink(defaultStatsdAddr)
	return initFR(ctx, sink, serviceName, l, opentracing.NoopTracer{})
}

func InitCli(ctx context.Context, name, logLevel string) (FlightRecorder, Closer) {
	format := "text"
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		format = "human"
	}

	l := logging.New(logLevel, logLevel, "", format).Named(name)
	// NOTE: set globals in deprecated.go, remove when deprecated is removed
	Log = l

	fr := NewFlightRecorder(name, metrics.Null, l, opentracing.NoopTracer{})
	return fr, func() {}
}

// InitFromEnv detects the environment it is running in (devbox or GCP) and invokes
// the appropriate Init funciton.
func InitFromEnv(ctx context.Context, name, logLevel string) (FlightRecorder, Closer) {
	if gce.OnGCE() {
		return InitGCP(ctx, name, logLevel)
	}
	return InitCli(ctx, name, logLevel)
}

func newStatsdSink(endpoint string) metrics.Sink {
	sink, err := metrics.NewStatsdSink(endpoint)
	if err != nil {
		panic(fmt.Errorf("error initializing metrics: %v", err))
	}
	return sink
}

func initFR(ctx context.Context, sink metrics.Sink, serviceName string, l logging.Logger, tr opentracing.Tracer) (FlightRecorder, Closer) {
	mr := metrics.NewReceiver(sink).ScopePrefix(serviceName)
	l = l.Named(serviceName)
	// NOTE: set globals in deprecated.go, remove when deprecated is removed
	Metrics = mr
	Log = l

	done := make(chan struct{})
	reportStandardMetrics(mr, done, l)

	fr := NewFlightRecorder(serviceName, mr, l, tr)
	// TODO: make this work. currently obs.logging uses SetOutput on the global logging which makes this a circlular dependency
	// log.SetOutput(stderrAdapter{fr.WithSpan(ctx)})

	return fr, func() {
		close(done)
		sink.Close()
	}
}

func reportStandardMetrics(mr metrics.Receiver, done <-chan struct{}, l logging.Logger) {
	reportGCMetrics(3*time.Second, done, mr)
	reportVersion(done, mr)
	reportUptime(done, mr)
	reportRusage(done, mr)
	reportFDCount(done, mr, l)
	reportThreadCount(done, mr, l)
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

func reportThreadCount(done <-chan struct{}, receiver metrics.Receiver, l logging.Logger) {
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				grNum := runtime.NumGoroutine()
				receiver.SetGauge("num_goroutines", float64(grNum))

				if nThreads, err := numOSThreads(); err != nil {
					l.Warn("numOSThreads failed", logging.Fields{}.WithError(err))
				} else {
					receiver.SetGauge("num_os_threads", float64(nThreads))
				}

				next = time.After(60 * time.Second)
			}
		}
	}()
}

func reportFDCount(done <-chan struct{}, receiver metrics.Receiver, l logging.Logger) {
	receiver = receiver.ScopePrefix("file_descriptors")
	go func() {
		next := time.After(0)
		for {
			select {
			case <-done:
				return
			case <-next:
				if nfds, err := numOpenFDs(); err != nil {
					l.Warn("numOpenFDs failed", logging.Fields{}.WithError(err))
				} else {
					receiver.SetGauge("open", float64(nfds))
				}

				if lim, err := maxOpenFDs(); err != nil {
					l.Warn("maxOpenFDs failed", logging.Fields{}.WithError(err))
				} else {
					receiver.SetGauge("limit", float64(lim))
				}

				next = time.After(60 * time.Second)
			}
		}
	}()
}

func numOpenFDs() (int, error) {
	fds, err := os.Open("/proc/self/fd")
	if err != nil {
		err = fmt.Errorf("open(/proc/self/fd): %v", err)
		return 0, err
	}
	defer fds.Close()

	names, err := fds.Readdirnames(0)
	if err != nil {
		err = fmt.Errorf("readdirnames(/proc/self/fd): %v", err)
		return 0, err
	}

	return len(names), nil
}

func numOSThreads() (int, error) {
	tasks, err := os.Open("/proc/self/task")
	if err != nil {
		err = fmt.Errorf("open(/proc/self/task): %v", err)
		return 0, err
	}
	defer tasks.Close()

	names, err := tasks.Readdirnames(0)
	if err != nil {
		err = fmt.Errorf("readdirnames(/proc/self/task): %v", err)
		return 0, err
	}
	return len(names), nil
}

func maxOpenFDs() (uint64, error) {
	var rlim syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	if err != nil {
		return 0, err
	}
	return rlim.Cur, nil
}

type stderrAdapter struct {
	fs FlightSpan
}

func (sa stderrAdapter) Write(bs []byte) (int, error) {
	sa.fs.Info(string(bs), Vals(getCallerContext(1)))
	return len(bs), nil
}
