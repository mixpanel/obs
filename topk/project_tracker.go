package topk

import (
	"log"
	"obs/metrics"
	"obs/mixpanel"
	"sync"
	"time"
)

const (
	CountTag        = "count"
	PreSamplingTag  = "pre_sampling"
	PostSamplingTag = "post_sampling"
	CETag           = "ce_event"
)

type ProjectTracker interface {
	Track(projectId int32, tags ...string)
	Close()
}

type NullProjectTracker struct{}

func (p *NullProjectTracker) Track(projectId int32, tags ...string) {}
func (p *NullProjectTracker) Close()                                {}

type projectCounts map[string]int64

type projectTracker struct {
	ticker    *time.Ticker
	client    mixpanel.Client
	eventName string
	receiver  metrics.Receiver

	mutex  sync.Mutex // guards everything below
	counts map[int32]projectCounts
}

func NewProjectTracker(client mixpanel.Client,
	receiver metrics.Receiver,
	flushInterval time.Duration,
	eventName string) ProjectTracker {
	p := &projectTracker{
		ticker:    time.NewTicker(flushInterval),
		client:    client,
		eventName: eventName,
		receiver:  receiver,
		counts:    make(map[int32]projectCounts),
	}

	go func() {
		for {
			select {
			case _, ok := <-p.ticker.C:
				if !ok {
					return
				}
				p.flush()
			}
		}
	}()

	return p
}

func (p *projectTracker) Track(projectId int32, tags ...string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.counts[projectId]; !ok {
		p.counts[projectId] = make(map[string]int64)
	}

	count := p.counts[projectId]
	count[CountTag]++
	for _, tag := range tags {
		count[tag]++
	}
}

func (p *projectTracker) send(events []*mixpanel.TrackedEvent) {
	err := p.client.TrackBatched(events)
	p.receiver.IncrBy("num_sent_events", float64(len(events)))
	if err != nil {
		log.Printf("error while tracking to mixpanel api: %v", err)
		p.receiver.Incr("failures")
	} else {
		p.receiver.Incr("success")
	}
}

func (p *projectTracker) flush() {
	p.mutex.Lock()
	counts := p.counts
	p.counts = make(map[int32]projectCounts, len(counts))
	p.mutex.Unlock()

	if len(counts) == 0 {
		return
	}

	var events []*mixpanel.TrackedEvent

	maxBatchSize := 100
	for projectId, count := range counts {
		props := map[string]interface{}{
			"distinct_id": projectId,
			"project_id":  projectId,
		}

		for k, v := range count {
			props[k] = v
		}

		events = append(events, &mixpanel.TrackedEvent{
			EventName:  p.eventName,
			Properties: props,
		})
		if len(events) == maxBatchSize {
			p.send(events)
			events = nil
		}
	}

	if len(events) > 0 {
		p.send(events)
	}
}

func (p *projectTracker) Close() {
	p.ticker.Stop()
	p.flush()
}
