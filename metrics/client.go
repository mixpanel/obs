package metrics

import (
	"bytes"
	"flags"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Tags map[string]string
type metricType string

var batchSize = 4096

var (
	metricTypeCounter = metricType("ct")
	metricTypeStat    = metricType("h")
	metricTypeGauge   = metricType("g")
)

type receiver struct {
	prefix  string
	metrics chan string
	tags    Tags
	wg      *sync.WaitGroup
}

type Receiver interface {
	Incr(name string)
	IncrBy(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	ScopePrefix(prefix string) Receiver
	ScopeTags(tags Tags) Receiver
	Scope(prefix string, tags Tags) Receiver

	StartStopwatch(name string) Stopwatch

	Close()
}

type stopwatch struct {
	name      string
	startTime time.Time
	receiver  Receiver
}

func (sw *stopwatch) Stop() {
	latency := time.Now().Sub(sw.startTime) / time.Microsecond
	sw.receiver.AddStat(fmt.Sprintf("%s_us", sw.name), float64(latency))
}

type Stopwatch interface {
	Stop()
}

func NewDefault() (Receiver, error) {
	address := flags.MetricsEndpoint
	if address != "" {
		return NewMetrics(address)
	} else {
		return Null, nil
	}
}

func NewMetrics(addr string) (Receiver, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	wg := &sync.WaitGroup{}
	metricsReceiver := &receiver{
		prefix:  "",
		metrics: make(chan string, 128),
		tags:    make(map[string]string),
		wg:      wg,
	}
	wg.Add(1)
	go metricsReceiver.flusher(conn)
	return metricsReceiver, nil
}

func (mr *receiver) send(name string, metricType metricType, value float64) {
	separator := ""
	if len(mr.prefix) > 0 {
		separator = "."
	}
	data := fmt.Sprintf("%s%s%s:%g|%s", mr.prefix, separator, name, value, metricType)

	// prefix.name:value|type|#tagKey:tagValue
	if len(mr.tags) > 0 {
		data += "|#"
		for k, v := range mr.tags {
			data += fmt.Sprintf("%s:%s,", k, v)
		}
		data = data[0 : len(data)-1]
	}

	mr.metrics <- data
}

func (mr *receiver) flusher(conn net.Conn) {
	buf := bytes.NewBuffer(nil)
	flushInterval := 5 * time.Second
	nextFlush := time.After(flushInterval)

	defer mr.wg.Done()
	for {
		select {
		case stat, ok := <-mr.metrics:
			if !ok {
				flushBuffer(conn, buf)
				return
			}

			io.WriteString(buf, stat)
			io.WriteString(buf, "\n")
			if buf.Len() > batchSize {
				flushBuffer(conn, buf)
			}
		case _ = <-nextFlush:
			flushBuffer(conn, buf)
			nextFlush = time.After(flushInterval)
		}
	}
}

func flushBuffer(conn net.Conn, buf *bytes.Buffer) {
	if buf.Len() > 0 {
		if _, err := conn.Write(buf.Bytes()); err != nil {
			log.Printf("error while writing to statsd: %v", err)
		}
		buf.Reset()
	}
}

func (mr *receiver) Incr(name string) {
	mr.IncrBy(name, 1)
}

func (mr *receiver) IncrBy(name string, amount float64) {
	mr.send(name, metricTypeCounter, amount)
}

func (mr *receiver) AddStat(name string, value float64) {
	mr.send(name, metricTypeStat, value)
}

func (mr *receiver) SetGauge(name string, value float64) {
	mr.send(name, metricTypeGauge, value)
}

func (mr *receiver) ScopePrefix(prefix string) Receiver {
	return mr.Scope(prefix, nil)
}

func (mr *receiver) ScopeTags(tags Tags) Receiver {
	return mr.Scope("", tags)
}

func (mr *receiver) Scope(prefix string, tags Tags) Receiver {
	newPrefix := prefix
	if len(prefix) == 0 {
		newPrefix = mr.prefix
	} else if len(mr.prefix) > 0 {
		newPrefix = fmt.Sprintf("%s.%s", mr.prefix, prefix)
	}

	newTags := make(map[string]string, len(tags)+len(mr.tags))
	for k, v := range mr.tags {
		newTags[k] = v
	}
	if tags != nil {
		for k, v := range tags {
			newTags[k] = v
		}
	}

	return &receiver{
		prefix:  newPrefix,
		metrics: mr.metrics,
		tags:    newTags,
	}
}

func (mr *receiver) StartStopwatch(name string) Stopwatch {
	return &stopwatch{
		name:      name,
		startTime: time.Now(),
		receiver:  mr,
	}
}

func (mr *receiver) Close() {
	close(mr.metrics)
	mr.wg.Wait()
}
