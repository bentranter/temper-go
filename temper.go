package temper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// Version is the release version of the Temper API client.
	Version = "0.0.5"

	defaultBaseURL = "https://temperhq.com"
)

type base struct {
	http    *http.Client
	baseURL string
}

type Client struct {
	base
	filter *filter
}

type Option struct {
	// The base URL of the Temper instance, defaults to https://temperhq.com.
	BaseURL string
}

func (o *Option) setDefaults() {
	if o.BaseURL == "" {
		o.BaseURL = defaultBaseURL
	}
}

type tokenSource struct {
	publishableKey string
	secretKey      string
	base           http.RoundTripper
}

// RoundTrip authorizes and authenticates the request with a publishable key
// when accessing the public filter API endpoint, and the secret key for all
// other endpoints.
func (ts *tokenSource) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBodyClosed := false
	if req.Body != nil {
		defer func() {
			if !reqBodyClosed {
				req.Body.Close()
			}
		}()
	}

	req2 := cloneRequest(req) // per RoundTripper contract
	if strings.HasPrefix(req2.URL.Path, "/api/public") {
		req2.Header.Set("Authorization", "Bearer "+ts.publishableKey)
	} else {
		req2.Header.Set("Authorization", "Bearer "+ts.secretKey)
	}

	// req.Body is assumed to be closed by the base RoundTripper.
	reqBodyClosed = true
	return ts.base.RoundTrip(req2)
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

// New creates a new instance of the Temper API client.
func New(publishableKey, secretKey string, opts ...*Option) *Client {
	publishableKey = strings.Trim(strings.TrimSpace(publishableKey), "'")
	if publishableKey == "" {
		log.Fatalln("go-temper: publishable key cannot be empty")
	}
	secretKey = strings.Trim(strings.TrimSpace(secretKey), "'")

	ts := &tokenSource{
		publishableKey: publishableKey,
		secretKey:      secretKey,
		base:           http.DefaultTransport,
	}

	httpClient := &http.Client{
		Transport: ts,
	}

	opt := &Option{}
	for _, o := range opts {
		opt = o
	}
	opt.setDefaults()

	common := &base{
		http:    httpClient,
		baseURL: opt.BaseURL,
	}

	client := &Client{
		base: *common,
	}

	if err := client.fetchFilter(); err != nil {
		log.Printf("go-temper: failed to fetch and intialize filter: %s, retrying in 60 seconds, all checks will return false", err.Error())
		client.filter = &filter{}
	}
	go client.pollFilter()

	return client
}

// fetchFilter gets the filter and rollout data from the Temper backend.
func (c *Client) fetchFilter() error {
	resp, err := c.http.Get(c.baseURL + "/api/public/filter")
	if err != nil {
		return fmt.Errorf("go-temper: failed to fetch filter: %w", err)
	}

	fr := &filterResponse{}
	if err := json.NewDecoder(resp.Body).Decode(fr); err != nil {
		return fmt.Errorf("go-temper: failed to decode filter response: %w", err)
	}
	defer resp.Body.Close()

	f, err := from(fr)
	if err != nil {
		return fmt.Errorf("go-temper: failed to create filter from data: %w", err)
	}
	c.filter = f

	return nil
}

func (c *Client) pollFilter() {
	for {
		time.Sleep(60 * time.Second)

		if err := c.fetchFilter(); err != nil {
			log.Printf("go-temper: latest filter poll failed at %s due to error: %s", time.Now().String(), err.Error())
		}
	}
}

// Check looks up a feature, returning true if it's enabled, and false
// otherwise.
func (c *Client) Check(feature string) bool {
	return c.filter.lookup([]byte(feature))
}
