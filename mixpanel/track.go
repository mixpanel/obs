package mixpanel

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Client interface {
	Track(e *TrackedEvent) error
	TrackBatched(es []*TrackedEvent) error
	Import(es []*TrackedEvent) error
	UrlWithTracking(e *TrackedEvent, dest string) (*url.URL, error)
}

type client struct {
	token   string
	apiKey  string
	baseUrl string
	api     *http.Client
}

type TrackedEvent struct {
	EventName  string
	DistinctID string
	Time       time.Time
	Properties map[string]interface{}
}

func NewClient(token, apiKey, baseUrl string) Client {
	return &client{
		token:   token,
		apiKey:  apiKey,
		baseUrl: baseUrl,
		api:     &http.Client{},
	}
}

func (c *client) TrackBatched(es []*TrackedEvent) error {
	return c.track(es)
}

func (c *client) Track(e *TrackedEvent) error {
	return c.track([]*TrackedEvent{e})
}

func (c *client) track(es []*TrackedEvent) error {
	if len(c.token) == 0 {
		return fmt.Errorf("token is empty")
	}

	for _, e := range es {
		if e.Time.IsZero() {
			e.Time = time.Now()
		}
	}

	data, err := c.encodeEvent(es)
	if err != nil {
		return err
	}

	params := make(url.Values)
	params.Set("data", data)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/track/", c.baseUrl), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return err
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("track returned status %d: %q", resp.Status, string(body))
	}
	_, _ = io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func (c *client) Import(events []*TrackedEvent) error {
	if len(c.token) == 0 || len(c.apiKey) == 0 {
		return fmt.Errorf("both token and API key must be specified")
	}
	data, err := c.encodeEvent(events)
	if err != nil {
		return err
	}

	params := make(url.Values)
	params.Set("data", data)
	params.Set("api_key", c.apiKey)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/import/", c.baseUrl), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return err
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("track returned status %d: %q", resp.Status, string(body))
	}
	_, _ = io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func (c *client) UrlWithTracking(event *TrackedEvent, dest string) (*url.URL, error) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}

	data, err := c.encodeEvent([]*TrackedEvent{event})
	if err != nil {
		return nil, err
	}

	trackCopy, err := url.ParseRequestURI(fmt.Sprintf("%s/track/", c.baseUrl))
	if err != nil {
		return nil, err
	}
	query := trackCopy.Query()
	query.Set("data", data)
	if dest != "" {
		query.Set("redirect", dest)
	}
	query.Set("ip", "1")
	trackCopy.RawQuery = query.Encode()

	return trackCopy, nil
}

func (c *client) encodeEvent(es []*TrackedEvent) (string, error) {
	var list []map[string]interface{}
	for _, e := range es {
		if e.EventName == "" {
			return "", fmt.Errorf("EventName cannot be empty")
		}

		properties := e.Properties
		if properties == nil {
			properties = make(map[string]interface{})
		}
		properties["time"] = e.Time.Unix()
		if len(e.DistinctID) != 0 {
			properties["distinct_id"] = e.DistinctID
		}
		properties["token"] = c.token

		list = append(list, map[string]interface{}{
			"event":      e.EventName,
			"properties": properties,
		})
	}

	jsonEncoded, err := json.Marshal(list)

	if err != nil {
		return "", err
	}

	base64Encoded := base64.StdEncoding.EncodeToString(jsonEncoded)
	return base64Encoded, nil
}
