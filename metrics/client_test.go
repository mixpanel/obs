package metrics

import (
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
	"util"

	"github.com/stretchr/testify/assert"
)

func init() {
	batchSize = 1
}

func TestCounterIncr(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	metrics.Incr("test_counter")
	assert.Equal(t, "test_counter:1|ct", sink.readAll())
}

func TestCounterIncrBy(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	metrics.IncrBy("test_counter", 123.456)
	assert.Equal(t, "test_counter:123.456|ct", sink.readAll())
}

func TestAddStat(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	metrics.AddStat("test_stat", 1.234)
	assert.Equal(t, "test_stat:1.234|h", sink.readAll())
}

func TestSetGauge(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	metrics.SetGauge("test_gauge", 4.321)
	assert.Equal(t, "test_gauge:4.321|g", sink.readAll())
}

func TestCounterWithTags(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(Tags{"aKey": "aValue", "aKey2": "aValue2"}).Incr("test_counter")
	assert.Equal(t, tags, parseTags(sink.readAll()))
}

func TestStatWithTags(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(Tags{"aKey": "aValue", "aKey2": "aValue2"}).AddStat("test_stat", 1.2345)
	assert.Equal(t, tags, parseTags(sink.readAll()))
}

func TestSetGaugeWithTags(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(tags).SetGauge("test_gauge", 4.321)
	assert.Equal(t, tags, parseTags(sink.readAll()))
}

func TestScopeOverridesTags(t *testing.T) {
	metrics, sink := newTestMetrics(t)
	tags1 := Tags{"aKey": "aValue1"}
	tags2 := Tags{"aKey": "aValue2"}

	metrics = metrics.ScopeTags(tags1)
	metrics.Incr("test")
	assert.Equal(t, tags1, parseTags(sink.readAll()))

	metrics = metrics.ScopeTags(tags2)
	metrics.Incr("test")
	assert.Equal(t, tags2, parseTags(sink.readAll()))
}

func TestScopePrefix(t *testing.T) {
	metrics, sink := newTestMetrics(t)

	metrics = metrics.ScopePrefix("prefix1")
	metrics.Incr("test")
	assert.Equal(t, "prefix1.test:1|ct", sink.readAll())

	metrics = metrics.ScopePrefix("prefix2")
	metrics.Incr("test")
	assert.Equal(t, "prefix1.prefix2.test:1|ct", sink.readAll())
}

func TestStopwatch(t *testing.T) {
	metrics, sink := newTestMetrics(t)

	sw := metrics.StartStopwatch("test_latency")
	time.Sleep(1 * time.Microsecond)
	sw.Stop()

	emitted := sink.readAll()
	re := regexp.MustCompile("\\Atest_latency_us:[0-9]+\\.?[0-9]*\\|h\\z")
	assert.True(t, re.MatchString(emitted))
}

type udpSink struct {
	address string
	conn    *net.UDPConn
}

func newUDPSink() *udpSink {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	util.CheckFatalError(err)
	conn, err := net.ListenUDP("udp", addr)
	util.CheckFatalError(err)

	return &udpSink{conn.LocalAddr().String(), conn}
}

func (sink *udpSink) readAll() string {
	result := make([]byte, 0, 128)
	buf := make([]byte, 128)
	for {
		sink.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		n, err := sink.conn.Read(buf)
		result = append(result, buf[0:n]...)
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			return strings.TrimSpace(string(result))
		}
		util.CheckFatalError(err)
	}
}

func newTestMetrics(t *testing.T) (Receiver, *udpSink) {
	sink := newUDPSink()
	metrics, err := NewMetrics(sink.address)
	assert.Nil(t, err)
	return metrics, sink
}

func parseTags(line string) Tags {
	tags := make(map[string]string)
	parts := strings.Split(line, "|")
	if len(parts) == 0 {
		return tags
	}
	tagPart := parts[len(parts)-1]
	if tagPart[0:1] != "#" {
		return tags
	}
	tagPairs := strings.Split(strings.TrimPrefix(tagPart, "#"), ",")
	for _, pair := range tagPairs {
		tuple := strings.Split(pair, ":")
		tags[tuple[0]] = tuple[1]
	}
	return tags
}
