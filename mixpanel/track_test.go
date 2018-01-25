package mixpanel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testServer struct {
	httpServer *httptest.Server

	mutex    sync.Mutex
	requests [][]byte
}

func newClient(token, apiKey, baseUrl string) *client {
	return &client{
		token:   token,
		apiKey:  apiKey,
		baseUrl: baseUrl,
		api:     &http.Client{},
	}
}

func newTestServer(wg *sync.WaitGroup) *testServer {
	server := &testServer{
		requests: make([][]byte, 0),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mutex.Lock()
		defer server.mutex.Unlock()

		body, _ := ioutil.ReadAll(r.Body)
		server.requests = append(server.requests, body)

		io.WriteString(w, "1")
		wg.Done()
	})

	server.httpServer = httptest.NewServer(handler)
	return server
}

func testEvents(t *testing.T, decoded []byte, expected []*TrackedEvent, token string, apiKey string) {
	var events []interface{}
	json.Unmarshal(decoded, &events)

	assert.Equal(t, len(expected), len(events))

	for i, e := range events {
		eventMap := e.(map[string]interface{})
		assert.Equal(t, expected[i].EventName, eventMap["event"])

		properties := eventMap["properties"].(map[string]interface{})
		assert.Equal(t, token, properties["token"])

		for key, value := range expected[i].Properties {
			if key == "time" {
				assert.Equal(t, value, int64(properties[key].(float64)))
			} else {
				assert.Equal(t, value, properties[key])
			}
		}
	}
}

func testRequestBody(t *testing.T, body []byte, expected []*TrackedEvent, token string, apiKey string) {
	values, err := url.ParseQuery(string(body))
	assert.Nil(t, err)

	assert.Equal(t, apiKey, values.Get("api_key"))
	escaped, err := url.QueryUnescape(values.Get("data"))
	assert.Nil(t, err)

	decoded, err := base64.StdEncoding.DecodeString(escaped)
	assert.Nil(t, err)

	testEvents(t, decoded, expected, token, apiKey)
}

func getEvents(numEvents int) []*TrackedEvent {
	events := make([]*TrackedEvent, numEvents)

	for i := 0; i < numEvents; i++ {
		events[i] = &TrackedEvent{
			EventName: fmt.Sprintf("some_event_%d", i),
			Properties: map[string]interface{}{
				"property": fmt.Sprintf("value_%d", i),
			},
		}
	}
	return events
}

func TestTrackBatched(t *testing.T) {
	events := getEvents(10)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	ts := newTestServer(wg)
	defer ts.httpServer.Close()

	client := newClient("some_token", "some_api_key", ts.httpServer.URL)

	client.TrackBatched(events)

	wg.Wait()

	assert.Equal(t, 1, len(ts.requests))

	testRequestBody(t, ts.requests[0], events, "some_token", "")
}

func TestTrack(t *testing.T) {
	events := getEvents(10)

	wg := &sync.WaitGroup{}
	wg.Add(10)
	ts := newTestServer(wg)
	defer ts.httpServer.Close()

	client := newClient("some_token", "some_api_key", ts.httpServer.URL)

	for _, e := range events {
		client.Track(e)
	}
	wg.Wait()

	assert.Equal(t, 10, len(ts.requests))
	for i := 0; i < 10; i++ {
		testRequestBody(t, ts.requests[i], []*TrackedEvent{events[i]}, "some_token", "")
	}
}

func TestImport(t *testing.T) {
	events := getEvents(10)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	ts := newTestServer(wg)
	defer ts.httpServer.Close()

	client := newClient("some_token", "some_api_key", ts.httpServer.URL)

	client.Import(events)
	wg.Wait()

	assert.Equal(t, 1, len(ts.requests))

	testRequestBody(t, ts.requests[0], events, "some_token", "some_api_key")
}

func TestEncodeEvent(t *testing.T) {
	events := getEvents(10)
	for _, e := range events {
		e.DistinctID = "some_distinct_id"
		e.Time = time.Now()
	}

	client := newClient("some_token", "some_api_key", "http://0:0")

	encoded, err := client.encodeEvent(events)
	assert.Nil(t, err)

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	assert.Nil(t, err)

	testEvents(t, decoded, events, "some_token", "")
}
