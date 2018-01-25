package metrics

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mixpanel/obs/util"
)

var batchSizeBytes = 4096

type statsdSink struct {
	metrics       chan *bytes.Buffer
	flushes       chan struct{}
	wg            *sync.WaitGroup
	conn          net.Conn
	flushInterval time.Duration
}

func (sink *statsdSink) Handle(metric string, tags Tags, value float64, metricType metricType) (err error) {
	buf := util.SharedBufferPool.Get()
	defer func() {
		if err != nil {
			util.SharedBufferPool.Put(buf)
		}
	}()

	if len(metric) == 0 {
		return errors.New("cannot handle empty metric")
	}

	// metric:value|type|#tag1:value1,tag2:value2
	// we use buf.WriteString instead of Fprintf because it's faster
	// as per documentation, WriteString never returns an error, so we ignore it here
	_, _ = buf.WriteString(metric)
	_, _ = buf.WriteString(":")
	if _, err := fmt.Fprintf(buf, "%g", value); err != nil {
		return err
	}
	_, _ = buf.WriteString("|")
	_, _ = buf.WriteString(string(metricType))

	if len(tags) > 0 {
		if _, err := buf.WriteString("|#"); err != nil {
			return err
		}
		numTags := len(tags)
		for k, v := range tags {
			_, _ = buf.WriteString(k)
			_, _ = buf.WriteString(":")
			_, _ = buf.WriteString(v)
			numTags--
			if numTags > 0 {
				_, _ = buf.WriteString(",")
			}
		}
	}

	sink.metrics <- buf
	return nil
}

func (sink *statsdSink) Flush() error {
	sink.flushes <- struct{}{}
	return nil
}

func (sink *statsdSink) flusher() {
	defer func() {
		if err := sink.conn.Close(); err != nil {
			log.Printf("error while closing connection to statsd: %v", err)
		}
		sink.wg.Done()
	}()

	nextFlush := time.After(sink.flushInterval)

	buffer := &bytes.Buffer{}
	flushBuffer := func() error {
		data := buffer.Next(buffer.Len())
		buffer.Reset()
		for written := 0; written < len(data); {
			n, err := sink.conn.Write(data[written:])
			if err != nil {
				log.Printf("error while writing to statsd: %v", err)
				return err
			}
			written += n
		}
		return nil
	}

	for {
		select {
		case stat := <-sink.metrics:
			writeStatToBuffer(stat, buffer)
			if buffer.Len() >= batchSizeBytes {
				flushBuffer()
			}
		case _, ok := <-sink.flushes:
			if !ok {
				// drain the metrics channel
				for {
					select {
					case stat := <-sink.metrics:
						writeStatToBuffer(stat, buffer)
					default:
						flushBuffer()
						return
					}
				}
			}
			flushBuffer()
		case _ = <-nextFlush:
			flushBuffer()
			nextFlush = time.After(sink.flushInterval)
		}
	}
}

func writeStatToBuffer(stat, buffer *bytes.Buffer) {
	_, _ = stat.WriteTo(buffer)
	_, _ = buffer.WriteString("\n")
	util.SharedBufferPool.Put(stat)
}

func (sink *statsdSink) Close() {
	close(sink.flushes)
	sink.wg.Wait()
}

func newStatsdSinkFromConn(conn net.Conn) (Sink, error) {
	wg := &sync.WaitGroup{}
	sink := &statsdSink{
		metrics:       make(chan *bytes.Buffer, 128),
		flushes:       make(chan struct{}),
		wg:            wg,
		conn:          conn,
		flushInterval: 5 * time.Second,
	}

	wg.Add(1)
	go sink.flusher()

	return sink, nil
}

// NewStatsdSink returns a Sink for statsd
// pass the address of the statsd daemon to it
func NewStatsdSink(addr string) (Sink, error) {
	if addr == "" {
		return &nullSink{}, nil
	}
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}

	return newStatsdSinkFromConn(conn)
}
