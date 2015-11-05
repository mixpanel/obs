package metrics

import (
	"flags"
	"fmt"
	"log"
	"net"
	"time"
)

type Tags map[string]string
type metricType string

var (
	metricTypeCounter = metricType("ct")
	metricTypeStat    = metricType("h")
	metricTypeGauge   = metricType("g")
)

type receiver struct {
	conn   net.Conn
	prefix string
	tags   Tags
}

type MetricsReceiver interface {
	Incr(name string)
	IncrBy(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	ScopePrefix(prefix string) MetricsReceiver
	ScopeTags(tags Tags) MetricsReceiver
	Scope(prefix string, tags Tags) MetricsReceiver

	StartStopwatch(name string) Stopwatch
}

type stopwatch struct {
	name      string
	startTime time.Time
	receiver  MetricsReceiver
}

func (sw *stopwatch) Stop() {
	latency := time.Now().Sub(sw.startTime) / time.Microsecond
	sw.receiver.AddStat(fmt.Sprintf("%s_us", sw.name), float64(latency))
}

type Stopwatch interface {
	Stop()
}

func NewDefault() (MetricsReceiver, error) {
	address := flags.MetricsEndpoint
	if address != "" {
		return NewMetrics(address)
	} else {
		return NullReceiver, nil
	}
}

func NewMetrics(addr string) (MetricsReceiver, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	metricsReceiver := &receiver{
		conn:   conn,
		prefix: "",
		tags:   make(map[string]string),
	}
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

	_, err := mr.conn.Write([]byte(data))
	if err != nil {
		log.Printf("Error while writing to statsd: %v", err)
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

func (mr *receiver) ScopePrefix(prefix string) MetricsReceiver {
	return mr.Scope(prefix, nil)
}

func (mr *receiver) ScopeTags(tags Tags) MetricsReceiver {
	return mr.Scope("", tags)
}

func (mr *receiver) Scope(prefix string, tags Tags) MetricsReceiver {
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
		conn:   mr.conn,
		prefix: newPrefix,
		tags:   newTags,
	}
}

func (mr *receiver) StartStopwatch(name string) Stopwatch {
	return &stopwatch{
		name:      name,
		startTime: time.Now(),
		receiver:  mr,
	}
}
