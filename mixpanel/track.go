package mixpanel

import (
	"bytes"
	"configmanager"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"obs/obserr"
	"sync"
	"time"
)

// maximum number of events /track will accept in a single POST
const MaxBatchSize = 50

type TrackDestination struct {
	TrackingToken    string `long:"query-tracking-token" required:"false" default:"" description:"MP access token for query metrics tracking"`
	TrackingEndpoint string `long:"query-tracking-endpoint" required:"false" default:"" description:"MP access endpoint for query metrics tracking"`
}

type MetricsTrackingSettings struct {
	TrackDestination
	SampleRate      float32 `float32:"subquery-tracking-sample-rate" required:"true" default:"1" description:"How many LQS subqueries are tracked out of all subqueries in percentage"`
	AdditionalDests []TrackDestination
}

var traceClientInstance Client
var traceClientOnce sync.Once

type Client interface {
	Track(e *TrackedEvent) error
	TrackBatched(es []*TrackedEvent) error
	Import(es []*TrackedEvent) error
	UrlWithTracking(e *TrackedEvent, dest string) (*url.URL, error)
}

type Null struct{}

func (n *Null) Track(*TrackedEvent) error {
	return nil
}

func (n *Null) TrackBatched([]*TrackedEvent) error {
	return nil
}

func (n *Null) Import([]*TrackedEvent) error {
	return nil
}

func (n *Null) UrlWithTracking(*TrackedEvent, string) (*url.URL, error) {
	return &url.URL{}, nil
}

type Project struct {
	Token   string `json:"token"`
	ApiKey  string `json:"api-key"`
	BaseUrl string `json:"base-url"`
}

func (td *TrackDestination) Project() Project {
	return Project{
		Token:   td.TrackingToken,
		BaseUrl: td.TrackingEndpoint,
	}
}

type client struct {
	projects []Project
	api      *http.Client
	cm       configmanager.Client
}

type TrackedEvent struct {
	EventName  string
	DistinctID string
	Time       time.Time
	Properties map[string]interface{}
}

type ClientOption func(c *client) *client

func Timeout(duration time.Duration) ClientOption {
	return func(c *client) *client {
		c.api.Timeout = duration
		return c
	}
}

func NewClient(token, apiKey, baseUrl string, opts ...ClientOption) Client {
	return NewClientWithProjects([]Project{Project{Token: token, ApiKey: apiKey, BaseUrl: baseUrl}}, opts...)
}

func NewClientWithConfigManager(cm configmanager.Client, opts ...ClientOption) Client {
	c := &client{
		cm: cm,
		api: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

func NewClientWithProjects(projects []Project, opts ...ClientOption) Client {
	c := &client{
		projects: projects,
		api: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

func NewClientWithProject(project Project, opts ...ClientOption) Client {
	return NewClientWithProjects([]Project{project}, opts...)
}

func (c *client) TrackBatched(es []*TrackedEvent) error {
	return c.track(es)
}

func (c *client) Track(e *TrackedEvent) error {
	return c.track([]*TrackedEvent{e})
}

func (c *client) validateProjects() ([]Project, error) {
	var projects []Project = c.projects
	if c.cm != nil {
		err := c.cm.Unmarshal("projects", &projects)
		if err != nil {
			return nil, obserr.Annotate(err, "unmarshal projects")
		}
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to track to")
	}
	if len(projects[0].Token) == 0 {
		return nil, fmt.Errorf("token is empty")
	}

	return projects, nil
}

func (c *client) track(es []*TrackedEvent) error {
	projects, err := c.validateProjects()
	if err != nil {
		return obserr.Annotate(err, "validate projects")
	}
	if len(es) > MaxBatchSize {
		return fmt.Errorf("%d exceeds maximum batch size %d", len(es), MaxBatchSize)
	}

	for _, e := range es {
		if e.Time.IsZero() {
			e.Time = time.Now()
		}
	}

	for _, p := range projects {
		data, err := p.encodeEvent(es)
		if err != nil {
			return obserr.Annotate(err, "encode event")
		}

		params := make(url.Values)
		params.Set("data", data)

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/track/", p.BaseUrl), bytes.NewBufferString(params.Encode()))
		if err != nil {
			return obserr.Annotate(err, "request create")
		}

		resp, err := c.api.Do(req)
		if err != nil {
			return obserr.Annotate(err, "request failed").Set("body", params.Encode())
		}
		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return obserr.Annotate(err, "read all")
			}
			return fmt.Errorf("track returned status %s: %q", resp.Status, string(body))
		}
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	return nil
}

func (c *client) Import(events []*TrackedEvent) error {
	projects, err := c.validateProjects()
	if err != nil {
		return err
	}

	for _, p := range projects {
		if len(p.Token) == 0 || len(p.ApiKey) == 0 {
			return fmt.Errorf("both token and API key must be specified")
		}
		data, err := p.encodeEvent(events)
		if err != nil {
			return err
		}

		params := make(url.Values)
		params.Set("data", data)
		params.Set("api_key", p.ApiKey)
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/import/", p.BaseUrl), bytes.NewBufferString(params.Encode()))
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
			return fmt.Errorf("track returned status %s: %q", resp.Status, string(body))
		}
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	return nil
}

func (c *client) UrlWithTracking(event *TrackedEvent, dest string) (*url.URL, error) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}

	projects, err := c.validateProjects()
	if err != nil {
		return nil, err
	}

	p := projects[0]
	data, err := p.encodeEvent([]*TrackedEvent{event})
	if err != nil {
		return nil, err
	}

	trackCopy, err := url.ParseRequestURI(fmt.Sprintf("%s/track/", p.BaseUrl))
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

func (p Project) encodeEvent(es []*TrackedEvent) (string, error) {
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
		properties["token"] = p.Token

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
