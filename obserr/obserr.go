package obserr

import (
	"errors"
	"fmt"
	"sync"
)

// Error should be used as a drop-in replacement for Golang's native error type
// where adding key/value data or annotated info would provide useful context making
// debugging easier. Error is safe to use concurrently.
//
// This should be used in conjunction with go/src/obs/flight_recorder.go's Vals type
// where the actual telemetry/reporting happens.
type Error struct {
	orig error

	mu   sync.RWMutex
	err  error
	vals map[string]interface{}
}

func (e *Error) deepCopy() *Error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &Error{
		orig: e.orig,
		mu:   sync.RWMutex{},
		err:  e.err,
		vals: e.Vals(),
	}
}

func New(e interface{}) *Error {
	var err error

	switch o := e.(type) {
	case string:
		err = errors.New(o)
	case *Error:
		return o.deepCopy()
	case error:
		err = o
	default:
		err = fmt.Errorf("%v", o)
	}

	return &Error{
		orig: err,
		err:  err,
		vals: make(map[string]interface{}),
	}
}

func (e *Error) Error() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.err.Error()
}

func (e *Error) Get(k string) interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.vals[k]
}

func (e *Error) Set(kvs ...interface{}) *Error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := 0; i < len(kvs); i += 2 {
		e.vals[kvs[i].(string)] = kvs[i+1]
	}
	return e
}

func (e *Error) Vals() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// TODO deep copy maps and slices?
	vals := make(map[string]interface{}, len(e.vals))
	for k, v := range e.vals {
		vals[k] = v
	}
	return vals
}

func (e *Error) Annotate(ann interface{}) *Error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var a string

	switch o := ann.(type) {
	case string:
		a = o
	case *Error:
		a = o.Error()
	case error:
		a = o.Error()
	default:
		a = fmt.Sprintf("%v", o)
	}

	e.err = fmt.Errorf("%s: %s", a, e.err)
	return e
}

func Annotate(e error, an interface{}) *Error {
	return New(e).Annotate(an)
}

func Original(e error) error {
	if oe, ok := e.(*Error); ok {
		// oe.orig read is safe because orig field is never changed after construction
		return oe.orig
	}
	return e
}
