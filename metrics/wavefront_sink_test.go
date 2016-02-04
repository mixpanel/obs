package metrics

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"util"

	"github.com/stretchr/testify/assert"
)

func TestWavefrontSinkWithoutTags(t *testing.T) {
	endpoint := newTcpEndpoint()
	endpoint.wg.Add(1)
	go newServer(endpoint)

	sink := newSink(endpoint.address)

	sink.Handle("test.metric", nil, 10, "ct")
	sink.Flush()

	endpoint.wg.Wait()

	split := strings.Split(strings.TrimSpace(string(endpoint.buf.Bytes())), " ")

	assert.Equal(t, len(split), 4)
	assert.Equal(t, "test.metric", split[0])
	assert.Equal(t, "10.000000", split[1])
	assert.Equal(t, "host=localhost", split[3])
}

func TestWavefrontSinkWithTags(t *testing.T) {
	endpoint := newTcpEndpoint()
	endpoint.wg.Add(1)
	go newServer(endpoint)

	sink := newSink(endpoint.address)

	tags := Tags{
		"a": "b",
		"c": "d",
	}

	sink.Handle("test.metric", tags, 10, "ct")
	sink.Flush()

	endpoint.wg.Wait()

	split := strings.Split(strings.TrimSpace(string(endpoint.buf.Bytes())), " ")

	assert.Equal(t, len(split), 6)
	assert.Equal(t, "test.metric", split[0])
	assert.Equal(t, "10.000000", split[1])
	assert.Equal(t, "host=localhost", split[3])

	mp := map[string]bool{
		"a=b": true,
		"c=d": true,
	}

	assert.False(t, split[4] == split[5])
	assert.True(t, mp[split[4]])
	assert.True(t, mp[split[5]])
}

type tcpEndpoint struct {
	address  string
	listener *net.TCPListener
	buf      *bytes.Buffer
	wg       *sync.WaitGroup
}

func newServer(endpoint *tcpEndpoint) {
	defer endpoint.wg.Done()
	conn, err := endpoint.listener.Accept()
	util.CheckFatalError(err)
	_, err = endpoint.buf.ReadFrom(conn)
	util.CheckFatalError(err)
}

func newSink(address string) Sink {
	return NewWavefrontSink("localhost", []string{address})
}

func newTcpEndpoint() *tcpEndpoint {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	util.CheckFatalError(err)
	listener, err := net.ListenTCP("tcp", addr)
	util.CheckFatalError(err)

	return &tcpEndpoint{
		address:  listener.Addr().String(),
		listener: listener,
		buf:      &bytes.Buffer{},
		wg:       &sync.WaitGroup{},
	}
}
