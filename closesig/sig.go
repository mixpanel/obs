package closesig

import (
	"fmt"
	"log"
	"net"
	"time"
)

const DefaultPort = 8126

func Client(port int) func() {
	const maxDelay = 1 * time.Minute
	done := make(chan struct{})
	go func() {
		dialer := net.Dialer{
			KeepAlive: 2 * time.Minute,
		}
		var delay time.Duration
		for {
			select {
			case <-time.After(delay):
				conn, err := dialer.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					delay += 100 * time.Millisecond
					if delay > maxDelay {
						delay = maxDelay
					}
					continue
				}
				<-done
				_ = conn.Close()
				return
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

type socketSig struct {
	srvAddr           net.Addr
	init, done, clear chan struct{}
}

func Server(port int) *socketSig {
	ss := &socketSig{
		init:  make(chan struct{}),
		done:  make(chan struct{}),
		clear: make(chan struct{}),
	}
	go ss.monitor(port)
	return ss
}

func (ss *socketSig) addr() net.Addr {
	<-ss.init
	return ss.srvAddr
}

func (ss *socketSig) Wait() {
	close(ss.done)
	<-ss.clear
}

func (ss *socketSig) monitor(port int) {
	defer close(ss.clear)

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Printf("socksig: error listening on %d", port)
		return
	}
	srv, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Printf("socksig: error listening on %d", port)
		return
	}

	ss.srvAddr = srv.Addr()
	close(ss.init)

	conn := make(chan *net.TCPConn, 1)

	go func() {
		defer close(conn)
		cn, err := srv.AcceptTCP()
		if err != nil {
			log.Printf("socksig: error accepting connection: %v", err)
			return
		}
		conn <- cn
	}()

	select {
	case cn, ok := <-conn:
		if !ok {
			return
		}
		b := make([]byte, 1)
		_, _ = cn.Read(b)
		return
	case <-ss.done:
		return
	}

}
