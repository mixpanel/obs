package topk

import (
	"log"
	"obs/metrics"
	"obs/mixpanel"
	"sync"
	"time"
)

type ProjectTracker interface {
	Track(projectId int32)
	Close()
}

type NullProjectTracker struct{}

func (p *NullProjectTracker) Track(projectId int32) {}
func (p *NullProjectTracker) Close()                {}

type projectTracker struct {
	ticker    *time.Ticker
	client    mixpanel.Client
	eventName string
	receiver  metrics.Receiver

	mutex  sync.Mutex // guards everything below
	counts map[int32]int64
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
		counts:    make(map[int32]int64),
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

func (p *projectTracker) Track(projectId int32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.counts[projectId]++
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
	p.counts = make(map[int32]int64, len(counts))
	p.mutex.Unlock()

	if len(counts) == 0 {
		return
	}

	var events []*mixpanel.TrackedEvent

	maxBatchSize := 100
	for projectId, count := range counts {
		events = append(events, &mixpanel.TrackedEvent{
			EventName: p.eventName,
			Properties: map[string]interface{}{
				"project_id": projectId,
				"count":      count,
			},
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
