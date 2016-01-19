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
			numTags -= 1
			if numTags > 0 {
				_, _ = buf.WriteString(",")
			}
		}
	}

	sink.metrics <- buf
	return nil
}

func (sink *statsdSink) Flush() error {
	sink.metrics <- nil
	return nil
}

func (sink *statsdSink) flusher() {
	nextFlush := time.After(sink.flushInterval)
	defer sink.wg.Done()

	buffer := &bytes.Buffer{}
	flushBuffer := func() error {
		if buffer.Len() > 0 {
			if _, err := sink.conn.Write(buffer.Bytes()); err != nil {
				log.Printf("error while writing to statsd: %v", err)
			}
			buffer.Reset()
		}
		return nil
	}

	for {
		select {
		case stat, ok := <-sink.metrics:
			if !ok {
				// channel is closed
				flushBuffer()
				return
			}

			if stat == nil {
				flushBuffer()
			} else {
				_, _ = stat.WriteTo(buffer)

				sharedBufferPool.put(stat)
				_, _ = buffer.WriteString("\n")

				if buffer.Len() > batchSizeBytes {
					flushBuffer()
				}
			}
		case _ = <-nextFlush:
			flushBuffer()
			nextFlush = time.After(sink.flushInterval)
		}
	}
}

func (sink *statsdSink) Close() {
	close(sink.metrics)
	sink.wg.Wait()
}

func NewStatsdSink(addr string) (Sink, error) {
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
