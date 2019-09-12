package metrics

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mixpanel/obs/util"

	"github.com/stretchr/testify/assert"
)

func TestStatsdSink(t *testing.T) {
	endpoint := newUdpEndpoint()
	sink := newStatsdSink(endpoint.address)
	endpoint.wg.Add(1)
	go newStatsdServer(endpoint)
	sink.Handle("test.metric", nil, 1, "ct")
	assert.Nil(t, sink.Flush())

	endpoint.wg.Wait()
	out := strings.TrimSpace(string(endpoint.buf.Bytes()))
	assert.Equal(t, "test.metric:1|ct", out)
}

func prefillPool(b *testing.B) {
	for n := 0; n < b.N; n++ {
		buf := &bytes.Buffer{}
		buf.Grow(256)
		util.SharedBufferPool.Put(buf)
	}
	b.ResetTimer()
}

func BenchmarkStatsdSinkSync(b *testing.B) {
	sink := newStatsdSinkNull()
	prefillPool(b)
	for n := 0; n < b.N; n++ {
		sink.Handle("test.metric", nil, 1, "ct")
	}
	_ = sink.Flush()
}

func BenchmarkStatsdSinkParallel(b *testing.B) {
	sink := newStatsdSinkNull()
	prefillPool(b)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sink.Handle("test.metric", nil, 1, "ct")
		}
	})
	_ = sink.Flush()
}

func BenchmarkStatsdSinkAsync(b *testing.B) {
	sink := newStatsdSinkNull()
	prefillPool(b)
	var wg sync.WaitGroup
	wg.Add(b.N)
	for n := 0; n < b.N; n++ {
		go func() {
			defer wg.Done()
			sink.Handle("test.metric", nil, 1, "ct")
		}()
	}
	wg.Wait()
	_ = sink.Flush()
}

type udpEndpoint struct {
	address  string
	listener *net.UDPConn
	buf      *bytes.Buffer
	wg       *sync.WaitGroup
}

func newStatsdServer(endpoint *udpEndpoint) {
	defer endpoint.wg.Done()
	b := make([]byte, 2048)
	n, _, err := endpoint.listener.ReadFromUDP(b)
	if err != nil {
		panic(err)
	}
	endpoint.buf.Write(b[:n])
}

func newStatsdServerLoop(endpoint *udpEndpoint) {
	defer endpoint.wg.Done()

	fmt.Printf("*** Listener start\n")
	for {
		b := make([]byte, 2048)
		n, _, err := endpoint.listener.ReadFromUDP(b)
		if err != nil {
			fmt.Printf("*** Listener exit: %v\n", err)
			break
		}
		endpoint.buf.Write(b[:n])
	}
}

func newStatsdSink(address string) Sink {
	sink, err := NewStatsdSink(address)
	if err != nil {
		panic(err)
	}
	return sink
}

func newUdpEndpoint() *udpEndpoint {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}

	return &udpEndpoint{
		address:  listener.LocalAddr().String(),
		listener: listener,
		buf:      &bytes.Buffer{},
		wg:       &sync.WaitGroup{},
	}
}

type nullAddr struct{}

func (nullAddr) Network() string {
	return "null"
}

func (nullAddr) String() string {
	return "null"
}

type nullConn struct{}

func (nullConn) Read(b []byte) (int, error) {
	return 0, nil
}

func (nullConn) Write(b []byte) (int, error) {
	return len(b), nil
}

func (nullConn) Close() error {
	return nil
}

func (nullConn) LocalAddr() net.Addr {
	return nullAddr{}
}

func (nullConn) RemoteAddr() net.Addr {
	return nullAddr{}
}

func (nullConn) SetDeadline(t time.Time) error {
	return nil
}

func (nullConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nullConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newStatsdSinkNull() Sink {
	conn := nullConn{}
	sink, err := NewStatsdSinkFromConn(conn)
	if err != nil {
		panic(err)
	}
	return sink
}
