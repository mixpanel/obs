package obs

import (
	"io"

	"golang.org/x/net/context"
)

type readCloser struct {
	rc    io.ReadCloser
	fr    FlightRecorder
	ctx   context.Context
	fs    FlightSpan
	done  DoneFunc
	total int64
}

// NewReadCloserWithSpan wraps an io.ReadCloser, starting the Span when Read is first called and annotating the Trace with the total bytes read when Close is called.
func NewReadCloserWithSpan(ctx context.Context, rc io.ReadCloser, fr FlightRecorder) io.ReadCloser {
	return &readCloser{fr: fr, rc: rc, ctx: ctx}
}

func (rc *readCloser) Read(p []byte) (int, error) {
	if rc.fs == nil {
		rc.fs, rc.ctx, rc.done = rc.fr.WithNewSpan(rc.ctx, "Read")
	}
	n, err := rc.rc.Read(p)
	rc.total += int64(n)
	return n, err
}

func (rc *readCloser) Close() error {
	err := rc.rc.Close()
	rc.fs.TraceSpan().SetTag("total_read", rc.total)
	rc.fs.TraceSpan().SetTag("close_error", err)
	rc.done()
	return err
}

type writeCloser struct {
	wc    io.WriteCloser
	fr    FlightRecorder
	ctx   context.Context
	fs    FlightSpan
	done  DoneFunc
	total int64
}

// NewWriteCloserWithSpan wraps an io.WriteCloser, starting the Span when Write is first called and annotating the Trace with the total bytes written when Close is called.
func NewWriteCloserWithSpan(ctx context.Context, wc io.WriteCloser, fr FlightRecorder) io.WriteCloser {
	return &writeCloser{fr: fr, wc: wc, ctx: ctx}
}

func (wc *writeCloser) Write(p []byte) (int, error) {
	if wc.fs == nil {
		wc.fs, wc.ctx, wc.done = wc.fr.WithNewSpan(wc.ctx, "Write")
	}
	n, err := wc.wc.Write(p)
	wc.total += int64(n)
	return n, err
}

func (wc *writeCloser) Close() error {
	err := wc.wc.Close()
	wc.fs.TraceSpan().SetTag("total_written", wc.total)
	wc.fs.TraceSpan().SetTag("close_error", err)
	wc.done()
	return err
}
