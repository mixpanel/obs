package metrics

import (
	"bytes"
	"net"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"util"

	"github.com/stretchr/testify/assert"
)

func init() {
	batchSizeBytes = 1
}

func BenchmarkHandleStats(b *testing.B) {
	ch := make(chan *bytes.Buffer, 16)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		remaining := b.N
		for _ = range ch {
			remaining--
			if remaining == 0 {
				wg.Done()
				return
			}
		}
	}()

	sink := &statsdSink{
		metrics: ch,
	}

	r := &receiver{
		prefix: "test",
		scopes: make(map[string]*receiver),
		sink:   sink,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.handle("test_counter", 1.0, metricTypeCounter)
	}
	wg.Wait()
}

func TestCounterIncr(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	metrics.Incr("test_counter")
	assert.Equal(t, "test_counter:1|ct", endpoint.readAll())
}

func TestCounterIncrBy(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	metrics.IncrBy("test_counter", 123.456)
	assert.Equal(t, "test_counter:123.456|ct", endpoint.readAll())
}

func TestAddStat(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	metrics.AddStat("test_stat", 1.234)
	assert.Equal(t, "test_stat:1.234|h", endpoint.readAll())
}

func TestSetGauge(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	metrics.SetGauge("test_gauge", 4.321)
	assert.Equal(t, "test_gauge:4.321|g", endpoint.readAll())
}

func TestCounterWithTags(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(Tags{"aKey": "aValue", "aKey2": "aValue2"}).Incr("test_counter")
	assert.Equal(t, tags, parseStatsdTags(endpoint.readAll()))
}

func TestStatWithTags(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(Tags{"aKey": "aValue", "aKey2": "aValue2"}).AddStat("test_stat", 1.2345)
	assert.Equal(t, tags, parseStatsdTags(endpoint.readAll()))
}

func TestSetGaugeWithTags(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	tags := Tags{"aKey": "aValue", "aKey2": "aValue2"}
	metrics.ScopeTags(tags).SetGauge("test_gauge", 4.321)
	assert.Equal(t, tags, parseStatsdTags(endpoint.readAll()))
}

func TestScopeOverridesTags(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)
	tags1 := Tags{"aKey": "aValue1"}
	tags2 := Tags{"aKey": "aValue2"}

	metrics = metrics.ScopeTags(tags1)
	metrics.Incr("test")
	assert.Equal(t, tags1, parseStatsdTags(endpoint.readAll()))

	metrics = metrics.ScopeTags(tags2)
	metrics.Incr("test")
	assert.Equal(t, tags2, parseStatsdTags(endpoint.readAll()))
}

func TestScopePrefix(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)

	metrics = metrics.ScopePrefix("prefix1")
	metrics.Incr("test")
	assert.Equal(t, "prefix1.test:1|ct", endpoint.readAll())

	metrics = metrics.ScopePrefix("prefix2")
	metrics.Incr("test")
	assert.Equal(t, "prefix1.prefix2.test:1|ct", endpoint.readAll())
}

func TestStopwatch(t *testing.T) {
	metrics, endpoint := newTestMetrics(t)

	sw := metrics.StartStopwatch("test_latency")
	time.Sleep(1 * time.Microsecond)
	sw.Stop()

	emitted := endpoint.readAll()
	re := regexp.MustCompile("\\Atest_latency_us:[0-9]+\\.?[0-9]*\\|h\\z")
	assert.True(t, re.MatchString(emitted))
}

type udpEndpoint struct {
	address string
	conn    *net.UDPConn
}

func newUDPEndpoint() *udpEndpoint {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	util.CheckFatalError(err)
	conn, err := net.ListenUDP("udp", addr)
	util.CheckFatalError(err)

	return &udpEndpoint{conn.LocalAddr().String(), conn}
}

func (endpoint *udpEndpoint) readAll() string {
	result := make([]byte, 0, 128)
	buf := make([]byte, 128)
	for {
		endpoint.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, err := endpoint.conn.Read(buf)
		result = append(result, buf[0:n]...)
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			return strings.TrimSpace(string(result))
		}
		util.CheckFatalError(err)
	}
}

func newTestMetrics(t *testing.T) (Receiver, *udpEndpoint) {
	endpoint := newUDPEndpoint()
	sink, err := NewStatsdSink(endpoint.address)
	assert.Nil(t, err)
	return NewReceiver(sink), endpoint
}

func parseStatsdTags(line string) Tags {
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
