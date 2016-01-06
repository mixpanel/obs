package metrics

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

var batchSizeBytes = 4096

type Sink interface {
	Handle(metric string, tags Tags, value float64, metricType metricType) error
	Close()
}

type nullSink struct{}

func (sink *nullSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	return nil
}

func (sink *nullSink) Close() {
}

var NullSink Sink = &nullSink{}

type statsdSink struct {
	metrics       chan *bytes.Buffer
	wg            *sync.WaitGroup
	conn          net.Conn
	flushInterval time.Duration
}

func (sink *statsdSink) Handle(metric string, tags Tags, value float64, metricType metricType) (err error) {
	buf := sharedBufferPool.get()
	defer func() {
		if err != nil {
			sharedBufferPool.put(buf)
		}
	}()

	if len(metric) == 0 {
		return errors.New("cannot write empty metric")
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
			numTags -= 1
			if numTags > 0 {
				_, _ = buf.WriteString(",")
			}
		}
	}

	sink.metrics <- buf
	return nil
}

func (sink *statsdSink) flusher() {
	buf := &bytes.Buffer{}
	nextFlush := time.After(sink.flushInterval)

	defer sink.wg.Done()

	flushBuffer := func(buf *bytes.Buffer) {
		if buf.Len() > 0 {
			if _, err := sink.conn.Write(buf.Bytes()); err != nil {
				log.Printf("error while writing to statsd: %v", err)
			}
			buf.Reset()
		}
	}

	for {
		select {
		case stat, ok := <-sink.metrics:
			if !ok {
				// channel is closed
				flushBuffer(buf)
				return
			}
			_, _ = stat.WriteTo(buf)
			sharedBufferPool.put(stat)
			_, _ = buf.WriteString("\n")

			if buf.Len() > batchSizeBytes {
				flushBuffer(buf)
			}
		case _ = <-nextFlush:
			flushBuffer(buf)
			nextFlush = time.After(sink.flushInterval)
		}
	}
}

func (sink *statsdSink) Close() {
	close(sink.metrics)
	sink.wg.Wait()
}

func NewSink(addr string) (Sink, error) {
	if addr == "" {
		return &nullSink{}, nil
	}
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}

	wg := &sync.WaitGroup{}
	sink := &statsdSink{
		metrics:       make(chan *bytes.Buffer, 128),
		wg:            wg,
		conn:          conn,
		flushInterval: 5 * time.Second,
	}

	wg.Add(1)
	go sink.flusher()

	return sink, nil
}
