package obs

import (
	"fmt"
	"obs/logging"
	"obs/metrics"
	"runtime"
	"time"
	"version"

	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// NewFlightRecorder constructs a new FlightRecorder with the underlying metrics, logger, and tracer.
func NewFlightRecorder(name string, metrics metrics.Receiver, logger logging.Logger, tracer opentracing.Tracer) FlightRecorder {
	return &flightRecorder{
		serviceName: name,
		name:        name,
		tags:        nil,

		mr: metrics,
		l:  logger,
		tr: tracer,
	}
}

// Tags should be used for categorizing telemetry. For example, you should use a Tag for something like
// query_type but not query_id. Typically the cardinality of values is small.
type Tags map[string]string

// Vals should be used for providing specific values in telemetry that are specific to that instance of
// data capture. For example, the specific project_id that a log message pertains to. If the cardinality
// of possible values is large, then Vals is the right type to use.
type Vals map[string]interface{}

// FlightRecorder is a unified interface for all types of telemetry. A FlightRecorder is opinionated about
// what is reported to each underlying system.
//
// To use a FlightRecorder, construct one with InitGCP or InitSoftlayer. This will return your root instance.
// Inject the instance into all users that need it. Each recipient of a FlightRecorder should immediately
// scope it with a name and tags if applicable. For example:
//
// func NewService() *Service {
//     return &Service{
//         fr: obs.FR.ScopeName("MyService"),
//     }
// }
//
// In order to report telemetry to a FlightRecorder, you must enter a span. Spans are derived from a
// context.Context. You can either reuse an existing Span or construct a new one. For example:
//
// func (s *Service) Do(ctx context.Context) {
//     fs := s.fr.WithSpan(ctx)
//
//     fs.Incr("thing_happened", 1)
//     fs.Info("something uninteresting happened")
//     s.doComplicatedThing(ctx)
// }
//
// func (s *Service) doComplicatedThing(ctx context.Context) {
//     fs, ctx, done := s.fr.WithNewSpan(ctx, "complicated_thing")
//     defer done()
//     fs.Incr("complicated_thing", 1)
//
// }

type DoneFunc func()

type FlightRecorder interface {
	// ScopeName returns a new FlightRecorder that will report telemetry scoped with the provided name.
	ScopeName(name string) FlightRecorder
	// ScopeTags returns a new FlightRecorder that will report telemetry scoped with the provided tags.
	ScopeTags(tags Tags) FlightRecorder
	// Scope returns a new FlightRecorder that will report telemetry scoped with the provided name and tags.
	Scope(name string, tags Tags) FlightRecorder

	// WithNewSpan returns a new FlightSpan to which telemetry can be reported, a context.Context that
	// can be used to propagate this Span, and a DoneFunc that should be called when the caller returns.
	// Typically, you want to use WithNewSpan to group telemetry into discrete meaningful operations, such
	// as service calls.
	WithNewSpan(ctx context.Context, opName string) (FlightSpan, context.Context, DoneFunc)

	// WithSpan returns a FlightSpan that reports into an existing Span that was created with WithNewSpan (or by
	// the underylying tracing system). Typically, you use this function instead of WithNewSpan if the function
	// is doing something minor, or doesn't represent a significant logical chunk of your application.
	WithSpan(ctx context.Context) FlightSpan

	// NoCtx() GlobalFlight
}

/*
type GlobalFlight interface {
	Debug(message string, vals Vals)
	Info(message string, vals Vals)

	Warn(name, message string, vals Vals)
	Critical(name, message string, vals Vals)

	Incr(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	StartStopwatch(name string) Stopwatch
}
*/

type FlightSpan interface {
	Trace(message string, vals Vals)
	Debug(message string, vals Vals)
	Info(message string, vals Vals)

	Warn(warnType, message string, vals Vals)
	Critical(critType, message string, vals Vals)

	Incr(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	StartStopwatch(name string) Stopwatch
}

type Stopwatch interface {
	Stop()
}

type flightRecorder struct {
	serviceName string
	name        string
	tags        Tags

	mr metrics.Receiver
	l  logging.Logger
	tr opentracing.Tracer
}

func (fr *flightRecorder) ScopeName(name string) FlightRecorder {
	return fr.Scope(name, nil)
}

func (fr *flightRecorder) ScopeTags(tags Tags) FlightRecorder {
	return fr.Scope("", tags)
}

func (fr *flightRecorder) Scope(name string, tags Tags) FlightRecorder {
	if len(name) == 0 && len(tags) == 0 {
		return fr
	}

	newName := fr.name + "." + name
	if len(fr.name) == 0 {
		newName = name
	} else if len(name) == 0 {
		newName = fr.name
	}

	frTags := make(Tags, len(tags)+len(fr.tags))
	frTags.update(fr.tags)
	frTags.update(tags)

	metricTags := make(metrics.Tags, len(tags))
	for k, v := range tags {
		metricTags[k] = v
	}

	return &flightRecorder{
		serviceName: fr.serviceName,
		name:        newName,
		tags:        frTags,
		mr:          fr.mr.Scope(name, metricTags),
		l:           fr.l.Named(newName),
	}
}

func (fr *flightRecorder) WithSpan(ctx context.Context) FlightSpan {
	span := opentracing.SpanFromContext(ctx)
	return &flightSpan{
		span:           span,
		ctx:            ctx,
		flightRecorder: fr,
	}
}

func (fr *flightRecorder) WithNewSpan(ctx context.Context, opName string) (FlightSpan, context.Context, DoneFunc) {
	var span opentracing.Span
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		span = fr.tr.StartSpan(opName, opentracing.ChildOf(parentSpan.Context()))
	} else {
		span = fr.tr.StartSpan(opName)
	}

	for k, v := range fr.tags {
		span = span.SetTag(k, v)
	}

	ctx = opentracing.ContextWithSpan(ctx, span)
	fs := &flightSpan{
		span:           span,
		ctx:            ctx,
		flightRecorder: fr,
	}
	return fs, ctx, span.Finish
}

type flightSpan struct {
	span opentracing.Span
	ctx  context.Context

	*flightRecorder
}

func (fs *flightSpan) logFields(vals Vals) logging.Fields {
	fields := make(logging.Fields, len(vals)+len(fs.tags))
	for k, v := range fs.tags {
		fields[k] = v
	}

	for k, v := range vals {
		fields[k] = v
	}

	fields["eventTime"] = time.Now().Format(time.RFC3339Nano)
	fields["serviceContext"] = map[string]interface{}{
		"service": fs.serviceName,
		"version": version.GitCommit,
	}

	fields["context"] = getCallerContext(3)
	return fields
}

func (fs *flightSpan) logTrace(message string, fields logging.Fields) {
	if fs.span != nil {
		fs.span.Log(opentracing.LogData{
			Event:   message,
			Payload: Vals(fields),
		})
	}
}

func (fs *flightSpan) Trace(message string, vals Vals) {
	fs.logTrace(message, fs.logFields(vals))
}

func (fs *flightSpan) Debug(message string, vals Vals) {
	fields := fs.logFields(vals)
	fs.l.Debug(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Info(message string, vals Vals) {
	fields := fs.logFields(vals)
	fs.l.Info(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Warn(name, message string, vals Vals) {
	fs.mr.ScopeTags(metrics.Tags{"error": "warning"}).IncrBy(name, 1)
	fields := fs.logFields(vals)
	fs.l.Warn(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Critical(name, message string, vals Vals) {
	fs.mr.ScopeTags(metrics.Tags{"error": "critical"}).IncrBy(name, 1)
	fields := fs.logFields(vals)
	fs.l.Error(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Incr(name string, amount float64) {
	fs.mr.IncrBy(name, amount)
	fs.logTrace(fmt.Sprintf("Incr %s, value: %g", name, amount), nil)
}

func (fs *flightSpan) AddStat(name string, value float64) {
	fs.mr.AddStat(name, value)
	fs.logTrace(fmt.Sprintf("AddStat %s, value: %g", name, value), nil)
}

func (fs *flightSpan) SetGauge(name string, value float64) {
	fs.mr.SetGauge(name, value)
	fs.logTrace(fmt.Sprintf("SetGauge %s, value: %g", name, value), nil)
}

func (fs *flightSpan) StartStopwatch(name string) Stopwatch {
	return &sw{name, fs, time.Now()}
}

type sw struct {
	name      string
	fs        *flightSpan
	startTime time.Time
}

func (s *sw) Stop() {
	latencyMicros := time.Now().Sub(s.startTime) / time.Microsecond
	s.fs.AddStat(s.name+"_us", float64(latencyMicros))
}

func (t Tags) update(r Tags) {
	for k, v := range r {
		t[k] = v
	}
}

// getCallerContext returns a map containing the filename, line number and function name
// that called the log message. the argument n is the number of callers to skip.
func getCallerContext(n int) map[string]interface{} {
	pc, file, line, ok := runtime.Caller(n)
	if !ok {
		return nil
	}
	funcName := runtime.FuncForPC(pc).Name()

	return map[string]interface{}{
		"reportLocation": map[string]interface{}{
			"filePath":     file,
			"lineNumber":   line,
			"functionName": funcName,
		},
	}
}
