package topk

import (
	"net/url"
	"obs/metrics"
	"obs/mixpanel"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	PreAndPostSampling = []string{PreSamplingTag, PostSamplingTag}
	PreSampling        = []string{PreSamplingTag}
)

type mockClient struct {
	Events []*mixpanel.TrackedEvent
	mutex  sync.Mutex
}

func (mock *mockClient) TrackBatched(es []*mixpanel.TrackedEvent) error {
	mock.mutex.Lock()
	defer mock.mutex.Unlock()

	for _, e := range es {
		mock.Events = append(mock.Events, e)
	}

	return nil
}

func (mock *mockClient) Track(e *mixpanel.TrackedEvent) error     { return nil }
func (mock *mockClient) Import(es []*mixpanel.TrackedEvent) error { return nil }
func (mock *mockClient) UrlWithTracking(e *mixpanel.TrackedEvent, dest string) (*url.URL, error) {
	return nil, nil
}

func newProjectTracker() (*projectTracker, *mockClient) {
	mockMpClient := &mockClient{
		Events: make([]*mixpanel.TrackedEvent, 0),
	}

	return &projectTracker{
		ticker:    time.NewTicker(10 * time.Second),
		client:    mockMpClient,
		receiver:  metrics.Null,
		eventName: "test_event",
		counts:    make(map[int32]projectCounts),
	}, mockMpClient
}

func testEvents(t *testing.T, projectIds []int32, tracker *projectTracker, client *mockClient, numEvents int) {
	client.Events = make([]*mixpanel.TrackedEvent, 0)

	for i := 0; i < numEvents; i++ {
		for _, p := range projectIds {
			if i%2 == 0 {
				tracker.Track(int32(p), PreAndPostSampling...)
			} else {
				tracker.Track(int32(p), PreSampling...)
			}
		}
	}

	tracker.flush()

	assert.Equal(t, len(projectIds), len(client.Events))

	eventMap := make(map[int32]bool)
	for _, e := range client.Events {
		eventMap[e.Properties["project_id"].(int32)] = true
		assert.Equal(t, int64(numEvents), e.Properties[CountTag])
		assert.Equal(t, int64(numEvents), e.Properties[PreSamplingTag])
		assert.Equal(t, int64(numEvents)/2, e.Properties[PostSamplingTag])
		assert.Equal(t, "test_event", e.EventName)
	}

	assert.Equal(t, len(projectIds), len(eventMap))
	for _, p := range projectIds {
		assert.Equal(t, true, eventMap[p])
	}
}

func TestProjectTracker(t *testing.T) {
	tracker, client := newProjectTracker()

	projectIds := make([]int32, 200)
	for i := 0; i < 200; i++ {
		projectIds[i] = int32(i)
	}

	testEvents(t, projectIds, tracker, client, 20)

	client.Events = make([]*mixpanel.TrackedEvent, 0)
	tracker.flush()
	assert.Equal(t, 0, len(client.Events))

	testEvents(t, projectIds, tracker, client, 30)
}
