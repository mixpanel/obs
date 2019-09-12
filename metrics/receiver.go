package metrics

import (
	"log"
	"sync"
	"time"
)

type Receiver interface {
	Incr(name string)
	IncrBy(name string, amount float64)
	AddStat(name string, value float64)
	SetGauge(name string, value float64)

	ScopePrefix(prefix string) Receiver
	ScopeTags(tags Tags) Receiver
	Scope(prefix string, tags Tags) Receiver

	StartStopwatch(name string) Stopwatch
}

type receiver struct {
	prefix string
	tags   Tags

	// guards 'scopes'
	lock   sync.RWMutex
	scopes map[string]*receiver

	sink Sink
}

var Null Receiver = &receiver{
	scopes: make(map[string]*receiver),
	sink:   NullSink,
}

func (r *receiver) handle(name string, value float64, metricType metricType) {
	if err := r.sink.Handle(formatName(r.prefix, name), r.tags, value, metricType); err != nil {
		log.Printf("error while handling metric type: %s. Error: %v", metricType, err)
	}
}

func (r *receiver) Incr(name string) {
	r.IncrBy(name, 1)
}

func (r *receiver) IncrBy(name string, amount float64) {
	r.handle(name, amount, metricTypeCounter)
}

func (r *receiver) AddStat(name string, value float64) {
	r.handle(name, value, metricTypeStat)
}

func (r *receiver) SetGauge(name string, value float64) {
	r.handle(name, value, metricTypeGauge)
}

func (r *receiver) ScopeTags(tags Tags) Receiver {
	return r.Scope("", tags)
}

func (r *receiver) ScopePrefix(prefix string) Receiver {
	return r.Scope(prefix, nil)
}

func (r *receiver) Scope(prefix string, tags Tags) Receiver {
	if prefix == "" && tags == nil {
		return r
	}

	tagsString := FormatTags(tags)

	key := prefix + "|" + tagsString

	r.lock.RLock()
	if val, ok := r.scopes[key]; ok {
		r.lock.RUnlock()
		return val
	}

	// key doesn't exist, update
	r.lock.RUnlock()
	newPrefix := formatName(r.prefix, prefix)
	newTags := make(map[string]string, len(tags)+len(r.tags))

	for k, v := range r.tags {
		newTags[k] = v
	}

	for k, v := range tags {
		newTags[k] = v
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if val, ok := r.scopes[key]; ok {
		return val
	}

	scoped := &receiver{
		prefix: newPrefix,
		tags:   newTags,
		scopes: make(map[string]*receiver),
		sink:   r.sink,
	}

	r.scopes[key] = scoped
	return scoped
}

func (r *receiver) StartStopwatch(name string) Stopwatch {
	return &stopwatch{
		name:      name,
		startTime: time.Now(),
		receiver:  r,
	}
}

func NewReceiver(sink Sink) Receiver {
	return &receiver{
		prefix: "",
		tags:   make(map[string]string),
		scopes: make(map[string]*receiver),
		sink:   sink,
	}
}
