package mixpanel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
)

var TrackUrl *url.URL

type Client struct {
	token string
}

type event struct {
	Name       string                 `json:"event"`
	Properties map[string]interface{} `json:"properties"`
}

func init() {
	var err error
	TrackUrl, err = url.ParseRequestURI("http://api.mixpanel.com/track/")
	if err != nil {
		log.Fatalf("cannot parse mixpanel TrackUrl: %s", err)
	}
}

func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token must not be empty")
	}
	return &Client{
		token,
	}, nil
}

func (c *Client) Track(eventName string, properties map[string]interface{}) bool {
	//TODO
	return false
}

// Returns the base64 encoded event json object
func (c *Client) EncodeEvent(eventName string, properties map[string]interface{}) (string, error) {
	if eventName == "" {
		return "", fmt.Errorf("eventName cannot be empty")
	}

	e := event{
		Name:       eventName,
		Properties: make(map[string]interface{}),
	}

	for k, v := range properties {
		e.Properties[k] = v
	}

	e.Properties["token"] = c.token

	jsonEncoded, err := json.Marshal(e)
	if err != nil {
		return "", err
	}

	base64Encoded := base64.URLEncoding.EncodeToString(jsonEncoded)
	return base64Encoded, nil
}

func (c *Client) UrlWithTracking(eventName string, properties map[string]interface{}, dest string) (*url.URL, error) {
	data, err := c.EncodeEvent(eventName, properties)
	if err != nil {
		return nil, err
	}

	trackCopy := *TrackUrl
	query := trackCopy.Query()
	query.Set("data", data)
	query.Set("redirect", dest)
	query.Set("ip", "1")
	trackCopy.RawQuery = query.Encode()

	return &trackCopy, nil
}
