package metrics

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/mixpanel/obs/util"
)

type wavefrontSink struct {
	origin    string
	tags      map[string]string
	hostPorts []string
	mutex     sync.Mutex // protects buffer and closed
	buffer    *bytes.Buffer
	closed    bool
}

func writeTags(buf *bytes.Buffer, tags Tags) {
	for k, v := range tags {
		buf.WriteString(k)
		buf.WriteString("=\"")
		buf.WriteString(v)
		buf.WriteString("\" ")
	}
}

func (sink *wavefrontSink) Handle(metric string, tags Tags, value float64, metricType metricType) error {
	if len(metric) == 0 {
		return errors.New("cannot handle empty metric")
	}

	buf := util.SharedBufferPool.Get()
	defer util.SharedBufferPool.Put(buf)

	// wavefront format: <metricName> <metricValue> [optionalTimestampInEpochSeconds] host=<host> [tag1=value1 tag2=value2 ... ]
	_, _ = buf.WriteString(metric)
	_, _ = buf.WriteString(" ")
	if _, err := fmt.Fprintf(buf, "%0.6f %d ", value, time.Now().Unix()); err != nil {
		return err
	}
	_, _ = buf.WriteString("host=")
	_, _ = buf.WriteString(sink.origin)
	_, _ = buf.WriteString(" ")

	writeTags(buf, tags)

	sinkTagsToWrite := make(Tags, len(sink.tags))
	for tag, value := range sink.tags {
		if _, ok := tags[tag]; !ok {
			sinkTagsToWrite[tag] = value
		}
	}
	writeTags(buf, sinkTagsToWrite)

	buf.WriteString("\n")

	sink.mutex.Lock()
	defer sink.mutex.Unlock()

	if sink.closed {
		return errors.New("sink is closed")
	}
	_, _ = buf.WriteTo(sink.buffer)
	return nil
}

func send(address string, data []byte) error {
	toSend := bytes.NewBuffer(data)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		e := fmt.Errorf("error while connecting to %s: %v", address, err)
		log.Printf(e.Error())
		return e
	}
	defer conn.Close()
	if _, err := toSend.WriteTo(conn); err != nil {
		e := fmt.Errorf("error while writing data to %s: %v", address, err)
		log.Printf(e.Error())
		return e
	}
	return nil
}

func (sink *wavefrontSink) Flush() error {
	sink.mutex.Lock()
	sendBuffer := &bytes.Buffer{}
	sink.buffer.WriteTo(sendBuffer)
	sink.buffer.Reset()
	sink.mutex.Unlock()

	if sendBuffer.Len() == 0 {
		return nil
	}

	var err error

	idx := rand.Intn(len(sink.hostPorts))
	for i := 0; i < len(sink.hostPorts); i++ {
		if err = send(sink.hostPorts[idx], sendBuffer.Bytes()); err == nil {
			return nil
		}
		idx = (idx + 1) % len(sink.hostPorts)
	}

	return err
}

func (sink *wavefrontSink) Close() {
	sink.mutex.Lock()
	sink.closed = true
	sink.mutex.Unlock()

	sink.Flush()
}

// NewWavefrontSink returns a sink for wavefront.
func NewWavefrontSink(origin string, tags map[string]string, hostPorts []string) Sink {
	return &wavefrontSink{
		origin:    origin,
		tags:      tags,
		hostPorts: hostPorts,
		buffer:    &bytes.Buffer{},
	}
}
