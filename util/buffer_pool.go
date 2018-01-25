package util

import "bytes"

var SharedBufferPool = newBufferPool()

type BufferPool chan *bytes.Buffer

func newBufferPool() BufferPool {
	return make(chan *bytes.Buffer, 128)
}

func (b BufferPool) Get() *bytes.Buffer {
	select {
	case buf := <-b:
		return buf
	default:
		return &bytes.Buffer{}
	}
}

func (b BufferPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	select {
	case b <- buf:
	default:
	}
}
