package obs

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/mixpanel/obs/logging"
	"github.com/mixpanel/obs/metrics"

	"google.golang.org/grpc"

	"context"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
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

		scoped: make(map[string]*flightRecorder),
	}
}

var NullFlightRecorder = NewFlightRecorder("null_recorder", metrics.Null, logging.Null, opentracing.NoopTracer{})
var NullFR = NullFlightRecorder
var NullFS = &nullFlightSpan{}

// Tags should be used for categorizing telemetry. For example, you should use a Tag for something like
// query_type but not query_id. Typically the cardinality of values is small.
type Tags map[string]string

// Vals should be used for providing specific values in telemetry that are specific to that instance of
// data capture. For example, the specific project_id that a log message pertains to. If the cardinality
// of possible values is large, then Vals is the right type to use.
type Vals map[string]interface{}

func (v Vals) Dupe() Vals {
	res := make(Vals, len(v))
	for key, val := range v {
		res[key] = val
	}
	return res
}

func (v Vals) Merge(m Vals) Vals {
	new := v.Dupe()
	for key, val := range m {
		new[key] = val
	}
	return new
}

type errWithVals interface {
	Vals() map[string]interface{}
}

func (v Vals) WithError(err error) Vals {
	res := v.Dupe()

	if e, ok := err.(errWithVals); ok {
		for k, val := range e.Vals() {
			res[k] = val
		}
	}

	code := grpc.Code(err)
	name := code.String()

	res["err"] = fmt.Sprintf("%v", err)
	res["grpc_code"] = fmt.Sprintf("%d: %s", code, name)
	return res
}

// FlightRecorder is a unified interface for all types of telemetry. A FlightRecorder is opinionated about
// what is reported to each underlying system.
//
// To use a FlightRecorder, construct one with InitGCP or InitSoftlayer. This will return your root instance.
// Inject the instance into all users that need it. Each recipient of a FlightRecorder should immediately
// scope it with a name and tags if applicable. For example:
//
// func NewService() *Service {
//     return &Service{
//         fr: obs.FR.ScopeName("my_service_snake_case"),
//     }
// }
//
// In order to report telemetry to a FlightRecorder, you must enter a span. Spans are derived from a
// context.Context. You can either reuse an existing Span or construct a new one. For example:
//
// func (s *Service) Do(ctx context.Context) {
//     fs := s.fr.WithSpan(ctx)
//
//     fs.Incr("thing_happened")
//     fs.Info("something uninteresting happened")
//     s.doComplicatedThing(ctx)
// }
//
// func (s *Service) doComplicatedThing(ctx context.Context) {
//     fs, ctx, done := s.fr.WithNewSpan(ctx, "complicated_thing")
//     defer done()
//     fs.Incr("complicated_thing")
// }
//
// Arguments into spans should be snake cased, using all lowercase and underscores
// This follow for counters, as well as warning / critical message types.
// Warnings and Criticals have built-in counters, there's no need to separately count
// For example:
//
// func (s *Service) someFunction(ctx context.Context) {
//     fs, ctx, done := s.fr.WithNewSpan(ctx, "some_function")
//     defer done()
//     if err := s.someSubFunction(); err != nil {
//         fs.Critical("some_function", "someFunction failed in someSubFunction", obs.Vals{}.WithErr(err))
//     }
//     fs.Incr("some_function.success")
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
	// Latency will be measured automatically by WithNewSpan, with the stop included inside the DoneFunc
	// Typically, you want to use WithNewSpan to group telemetry into discrete meaningful operations, such
	// as service calls.
	WithNewSpan(ctx context.Context, opName string) (FlightSpan, context.Context, DoneFunc)

	// WithSpan returns a FlightSpan that reports into an existing Span that was created with WithNewSpan (or by
	// the underylying tracing system). Typically, you use this function instead of WithNewSpan if the function
	// is doing something minor, or doesn't represent a significant logical chunk of your application.
	WithSpan(ctx context.Context) FlightSpan

	// GRPCClient returns a grpc.DialOption to use to allow this FlightRecorder to intercept and instrument
	// Unary RPCs with that particular client. Make sure to also include GRPCStreamClient.
	GRPCClient() grpc.DialOption

	// GRPCStreamClient returns a grpc.DialOption to use to allow this FlightRecorder to intercept and instrument
	// streaming RPCs with that particular client. Make sure to also include GRPClient.
	GRPCStreamClient() grpc.DialOption

	// GRPCServer returns a grpc.ServerOption to use to allow this FlightRecorder to intercept and instrument
	// unary RPCs with that particular server. Make sure to also include GRPCStreamServer.
	GRPCServer() grpc.ServerOption

	// GRPCStreamServer returns a grpc.ServerOption to use to allow this FlightRecorder to intercept and instrument
	// streaming RPCs with that particular server. Make sure to also include GRPServer.
	GRPCStreamServer() grpc.ServerOption

	// GRPCServer returns a grpc.UnaryServerInterceptor to use to allow this FlightRecorder to intercept and instrument
	// unary RPCs with that particular server. Make sure to also include grpc.StreamServerInterceptor.
	GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor

	// GRPCServer returns a grpc.StreamServerInterceptor to use to allow this FlightRecorder to intercept and instrument
	// unary RPCs with that particular server. Make sure to also include grpc.UnaryServerInterceptor.
	GRPCStreamServerInterceptor() grpc.StreamServerInterceptor

	// WithNewSpanContext is like WithNewSpan but allows you to specify the parent SpanContext instead of deriving it
	// from the context.Context. This is usually only useful for libraries that derive tracing contexts from out-of-process
	// origins, such as as GRPC request where the tracing context is embeded in GRPC Metadata.
	WithNewSpanContext(ctx context.Context, opName string, spanCtx opentracing.SpanContext) (FlightSpan, context.Context, DoneFunc)

	// WithRootSpan is like WithNewSpan but allows you to force a root span and set its sample rate.
	WithRootSpan(ctx context.Context, opName string, sampleOneInN int) (FlightSpan, context.Context, DoneFunc)

	GetReceiver() metrics.Receiver
}

type FlightSpan interface {
	Trace(message string, vals Vals)
	Debug(message string, vals Vals)
	Info(message string, vals Vals)

	Warn(warnType, message string, vals Vals)
	Critical(critType, message string, vals Vals)

	Incr(name string)
	IncrBy(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	StartStopwatch(name string) Stopwatch

	TraceSpan() opentracing.Span
	TraceID() (string, bool)
	Sampled() bool
	SetTraceHeaders(http.Header) error
}

type Stopwatch interface {
	Elapsed() time.Duration
	Stop()
}

type flightRecorder struct {
	serviceName string
	name        string
	tags        Tags

	mr metrics.Receiver
	l  logging.Logger
	tr opentracing.Tracer

	mu     sync.Mutex
	scoped map[string]*flightRecorder
}

func (fr *flightRecorder) ScopeName(name string) FlightRecorder {
	return fr.Scope(name, nil)
}

func (fr *flightRecorder) ScopeTags(tags Tags) FlightRecorder {
	return fr.Scope("", tags)
}

func joinNames(lhs, rhs string) string {
	if len(lhs) == 0 {
		return rhs
	} else if len(rhs) == 0 {
		return lhs
	}
	return lhs + "." + rhs
}

func (fr *flightRecorder) GetReceiver() metrics.Receiver {
	return fr.mr
}

func (fr *flightRecorder) GRPCClient() grpc.DialOption {
	return grpc.WithUnaryInterceptor(tracingUnaryClientInterceptor(fr, fr.tr))
}

func (fr *flightRecorder) GRPCStreamClient() grpc.DialOption {
	return grpc.WithStreamInterceptor(tracingStreamClientInterceptor(fr, fr.tr))
}

func (fr *flightRecorder) GRPCServer() grpc.ServerOption {
	return grpc.UnaryInterceptor(TracingUnaryServerInterceptor(fr, fr.tr))
}
func (fr *flightRecorder) GRPCStreamServer() grpc.ServerOption {
	return grpc.StreamInterceptor(tracingStreamServerInterceptor(fr, fr.tr))
}

func (fr *flightRecorder) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return TracingUnaryServerInterceptor(fr, fr.tr)
}

func (fr *flightRecorder) GRPCStreamServerInterceptor() grpc.StreamServerInterceptor {
	return tracingStreamServerInterceptor(fr, fr.tr)
}

func (fr *flightRecorder) mkScoped(name string, tags Tags) *flightRecorder {
	newName := joinNames(fr.name, name)

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

		mr: fr.mr.Scope(name, metricTags),
		l:  fr.l.Named(newName),
		tr: fr.tr,

		scoped: make(map[string]*flightRecorder),
	}
}

func (fr *flightRecorder) Scope(name string, tags Tags) FlightRecorder {
	if len(name) == 0 && len(tags) == 0 {
		return fr
	}

	// can't cache if there are tags
	if tags != nil {
		return fr.mkScoped(name, tags)
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()
	if sfr, ok := fr.scoped[name]; ok {
		return sfr
	}
	sfr := fr.mkScoped(name, tags)
	fr.scoped[name] = sfr
	return sfr
}

func (fr *flightRecorder) WithSpan(ctx context.Context) FlightSpan {
	span := opentracing.SpanFromContext(ctx)
	return &flightSpan{
		span:           span,
		ctx:            ctx,
		flightRecorder: fr,
	}
}

func (fs *flightSpan) TraceSpan() opentracing.Span {
	if fs.span == nil {
		return noopSpan
	}
	return fs.span
}

func (fs *flightSpan) SetTraceHeaders(h http.Header) error {
	if fs.span == nil {
		return nil
	}

	if sctx, ok := fs.span.Context().(basictracer.SpanContext); ok {
		tracer := fs.span.Tracer()
		return tracer.Inject(sctx, opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(h))
	}

	return nil
}

func (fr *flightRecorder) WithNewSpan(ctx context.Context, opName string) (FlightSpan, context.Context, DoneFunc) {
	var spanCtx opentracing.SpanContext
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		spanCtx = parentSpan.Context()
	}
	return fr.WithNewSpanContext(ctx, opName, spanCtx)
}

func (fr *flightRecorder) WithNewSpanContext(ctx context.Context, opName string, spanCtx opentracing.SpanContext) (FlightSpan, context.Context, DoneFunc) {
	var span opentracing.Span
	fullOpName := joinNames(fr.name, opName)
	if spanCtx != nil {
		span = fr.tr.StartSpan(fullOpName, opentracing.ChildOf(spanCtx))
	} else {
		span = fr.tr.StartSpan(fullOpName)
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
	sw := fs.StartStopwatch(opName + ".latency")
	return fs, ctx, func() {
		sw.Stop()
		span.Finish()
	}
}

func (fr *flightRecorder) WithRootSpan(ctx context.Context, opName string, sampleOneInN int) (FlightSpan, context.Context, DoneFunc) {
	fs, ctx, done := fr.WithNewSpanContext(ctx, opName, nil)

	if sc, ok := fs.TraceSpan().Context().(basictracer.SpanContext); ok {
		if sc.TraceID%uint64(sampleOneInN) == 0 {
			ext.SamplingPriority.Set(fs.TraceSpan(), 1)
		} else {
			ext.SamplingPriority.Set(fs.TraceSpan(), 0)
		}
	}
	return fs, ctx, done
}

type flightSpan struct {
	span opentracing.Span
	ctx  context.Context

	*flightRecorder
}

func (fs *flightSpan) TraceID() (string, bool) {
	if fs.span == nil {
		return "", false
	}
	if id, ok := fs.span.Context().(basictracer.SpanContext); ok {
		return fmt.Sprintf("%032x", id.TraceID), true
	}
	return "", false
}

func (fs *flightSpan) Sampled() bool {
	if fs.span == nil {
		return false
	}
	if sc, ok := fs.span.Context().(basictracer.SpanContext); ok {
		return sc.Sampled
	}
	return false
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
		// TODO try add version info
		//"version": version.GitCommit,
	}

	fields["context"] = getCallerContext(3)
	if traceID, ok := fs.TraceID(); ok {
		fields["trace_id"] = traceID
	}
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
	fs.mr.ScopeTags(metrics.Tags{"error": "warning"}).IncrBy(name+".warning", 1)
	fields := fs.logFields(vals)
	fields["warning_log_name"] = name
	fs.l.Warn(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Critical(name, message string, vals Vals) {
	fs.mr.ScopeTags(metrics.Tags{"error": "critical"}).IncrBy(name+".critical_error", 1)
	fields := fs.logFields(vals)
	fields["critical_log_name"] = name
	fs.l.Error(message, fields)
	fs.logTrace(message, fields)
}

func (fs *flightSpan) Incr(name string) {
	fs.IncrBy(name, 1)
}

func (fs *flightSpan) IncrBy(name string, amount float64) {
	fs.mr.IncrBy(name, amount)
	// NOTE: We don't normally actually log these messages anywhere,
	// but formatting them accounts for almost 10% of our time spent
	// in metrics (and not-logging for another 1%) in the main code
	// (and the majority of our fake-tracking time in a few tools).
	// If this turns out to lose important information, at least
	// uncomment the if below, which will skip the wasted work in a
	// large subset of the cases. (To really catch all of them, you
	// also need to see if the raw span is nil or its context is not
	// sampling or a do-nothing recorder is hooked up to it.) -- AB
	// 20Apr2018 SYS-3786
	// if fs.span != nil && fs.span.Tracer() != nil {
	//	fs.logTrace(fmt.Sprintf("Incr %s, value: %g", name, amount), nil)
	// }
}

func (fs *flightSpan) AddStat(name string, value float64) {
	fs.mr.AddStat(name, value)
	// NOTE: See above (IncrBy, SYS-3786)
	// fs.logTrace(fmt.Sprintf("AddStat %s, value: %g", name, value), nil)
}

func (fs *flightSpan) SetGauge(name string, value float64) {
	fs.mr.SetGauge(name, value)
	// NOTE: See above (IncrBy, SYS-3786)
	// fs.logTrace(fmt.Sprintf("SetGauge %s, value: %g", name, value), nil)
}

func (fs *flightSpan) StartStopwatch(name string) Stopwatch {
	return &sw{name, fs, time.Now()}
}

type sw struct {
	name      string
	fs        *flightSpan
	startTime time.Time
}

func (s *sw) Elapsed() time.Duration {
	return time.Since(s.startTime)
}

func (s *sw) Stop() {
	d := s.Elapsed()
	s.fs.AddStat(s.name+"_us", float64(d/time.Microsecond))
	s.fs.TraceSpan().SetTag(s.name, d.String())
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

var noopSpan = opentracing.NoopTracer{}.StartSpan("noop")

type nullFlightSpan struct {
}

func (fs *nullFlightSpan) TraceID() (string, bool) {
	return "", false
}

func (fs *nullFlightSpan) Sampled() bool {
	return false
}

func (fs *nullFlightSpan) Trace(message string, vals Vals) {
}

func (fs *nullFlightSpan) Debug(message string, vals Vals) {
}

func (fs *nullFlightSpan) Info(message string, vals Vals) {
}

func (fs *nullFlightSpan) Warn(name, message string, vals Vals) {
}

func (fs *nullFlightSpan) Critical(name, message string, vals Vals) {
}

func (fs *nullFlightSpan) Incr(name string) {
}

func (fs *nullFlightSpan) IncrBy(name string, amount float64) {
}

func (fs *nullFlightSpan) AddStat(name string, value float64) {
}

func (fs *nullFlightSpan) SetGauge(name string, value float64) {
}

func (fs *nullFlightSpan) SetTraceHeaders(h http.Header) error {
	return nil
}

func (fs *nullFlightSpan) TraceSpan() opentracing.Span {
	return noopSpan
}

func (fs *nullFlightSpan) StartStopwatch(name string) Stopwatch {
	return &nullSW{}
}

type nullSW struct {
}

func (s *nullSW) Elapsed() time.Duration {
	return 0
}

func (s *nullSW) Stop() {
}
