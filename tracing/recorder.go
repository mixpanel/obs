package tracing

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/compute/metadata"

	"golang.org/x/oauth2/google"

	basictracer "github.com/opentracing/basictracer-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	cloudtrace "google.golang.org/api/cloudtrace/v1"
)

func New(opts basictracer.Options) opentracing.Tracer {
	opts.Recorder = newRecorder()
	return basictracer.NewWithOptions(opts)
}

func newRecorder() *recorder {
	client, err := google.DefaultClient(context.Background(), cloudtrace.TraceAppendScope)
	if err != nil {
		log.Printf("error initializing google.DefaultClient: %v", err)
		return &recorder{}
	}
	// TODO: If the gRPC client is available, use that. It's not available as of 10/18/2016.
	service, err := cloudtrace.New(client)
	if err != nil {
		log.Printf("error initializing cloudtrace Service: %v", err)
		return &recorder{}
	}

	project, err := metadata.ProjectID()
	if err != nil {
		log.Printf("error retrieving GCP project: %v", err)
		return &recorder{}
	}

	r := &recorder{
		svc:     cloudtrace.NewProjectsService(service),
		traces:  make(chan basictracer.RawSpan, 64),
		project: project,
	}

	go func() {
		const spanBufferSize = 128
		buf := make([]basictracer.RawSpan, 0, spanBufferSize)
		var tick <-chan time.Time

		flush := func() {
			tick = nil
			if len(buf) == 0 {
				return
			}
			r.flushSpans(buf)
			buf = buf[:0]
		}

		for {
			select {
			case <-tick:
				flush()

			case trace := <-r.traces:
				buf = append(buf, trace)

				if len(buf) == cap(buf) {
					// need to flush immediately, to avoid the buffer from resizing
					flush()
				}

				if tick == nil && len(buf) > 0 {
					tick = time.After(3 * time.Second)
				}
			}
		}
	}()
	return r

}

type recorder struct {
	svc     *cloudtrace.ProjectsService
	traces  chan basictracer.RawSpan
	project string
}

func (r *recorder) RecordSpan(raw basictracer.RawSpan) {
	if r.svc == nil {
		return
	}

	if !raw.Context.Sampled {
		return
	}
	r.traces <- raw
}

func (r *recorder) flushSpans(spans []basictracer.RawSpan) {
	traces := make([]*cloudtrace.Trace, len(spans))
	for i := range spans {
		traces[i] = r.rawSpanToTrace(spans[i])
	}

	_, err := r.svc.PatchTraces(r.project, &cloudtrace.Traces{Traces: traces}).Do()

	if err != nil {
		log.Printf("error sending trace to cloudtrace: %v", err)
	}
}

func (r *recorder) rawSpanToTrace(raw basictracer.RawSpan) *cloudtrace.Trace {
	span := &cloudtrace.TraceSpan{
		EndTime:      raw.Start.Add(raw.Duration).Format(time.RFC3339Nano),
		Kind:         extractKind(raw),
		Labels:       formatTags(raw.Tags),
		Name:         raw.Operation,
		ParentSpanId: raw.ParentSpanID,
		SpanId:       raw.Context.SpanID,
		StartTime:    raw.Start.Format(time.RFC3339Nano),
	}
	return &cloudtrace.Trace{
		ProjectId: r.project,
		TraceId:   fmt.Sprintf("%032x", raw.Context.TraceID),
		Spans:     []*cloudtrace.TraceSpan{span},
	}
}

func formatTags(tags opentracing.Tags) map[string]string {
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

func extractKind(raw basictracer.RawSpan) string {
	switch raw.Tags[string(ext.SpanKind)] {
	case ext.SpanKindRPCClientEnum:
		return "RPC_CLIENT"
	case ext.SpanKindRPCServerEnum:
		return "RPC_SERVER"
	default:
		return "SPAN_KIND_UNSPECIFIED"
	}
}
