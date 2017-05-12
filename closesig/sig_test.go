package closesig

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSocketSigNeverCreated(t *testing.T) {
	ss := Server(0)
	addr := ss.addr()
	assert.NotNil(t, addr)
	ss.Wait()
	assertClosed(t, ss)
}

func TestSocketSigConnected(t *testing.T) {
	ss := Server(0)
	addr := ss.addr()
	assert.NotNil(t, addr)
	_, port, err := net.SplitHostPort(addr.String())
	assert.NoError(t, err)
	p, err := strconv.Atoi(port)
	assert.NoError(t, err)
	done := Client(p)
	done()
	ss.Wait()
	assertClosed(t, ss)
}

func assertClosed(t *testing.T, ss *socketSig) {
	select {
	case _, ok := <-ss.clear:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Error("expecting channel to be closed")
	}
}

func assertNotClosed(t *testing.T, ss *socketSig) {
	select {
	case <-ss.clear:
		t.Error("expecting channel to not be closed")
	case <-time.After(50 * time.Millisecond):
	}
}
