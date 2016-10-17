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

func New() opentracing.Tracer {
	return basictracer.New(newRecorder())
}

func newRecorder() *recorder {
	client, err := google.DefaultClient(context.Background(), cloudtrace.TraceAppendScope)
	if err != nil {
		log.Printf("error initializing google.DefaultClient: %v", err)
		return &recorder{}
	}
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

	return &recorder{
		svc:     cloudtrace.NewProjectsService(service),
		project: project,
	}
}

type recorder struct {
	svc     *cloudtrace.ProjectsService
	project string
}

func (r *recorder) RecordSpan(raw basictracer.RawSpan) {
	if r.svc == nil {
		return
	}

	// TODO: batch
	_, err := r.svc.PatchTraces(r.project, &cloudtrace.Traces{
		Traces: []*cloudtrace.Trace{r.rawSpanToTrace(raw)},
	}).Do()

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
