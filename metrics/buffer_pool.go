package metrics

import "bytes"

var sharedBufferPool = newBufferPool()

type bufferPool chan *bytes.Buffer

func newBufferPool() bufferPool {
	return make(chan *bytes.Buffer, 128)
}

func (b bufferPool) get() *bytes.Buffer {
	select {
	case buf := <-b:
		return buf
	default:
		return &bytes.Buffer{}
	}
}

func (b bufferPool) put(buf *bytes.Buffer) {
	buf.Reset()
	select {
	case b <- buf:
	default:
	}
}
