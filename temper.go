package temper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"sync"
)

const (
	// Version is the release version of the Temper API client.
	Version = "0.0.6"

	defaultBaseURL = "https://temperhq.com"
)

var (
	// c contains the one and only instance of client.
	c *client

	// once is used to ensure the client instance is only ever initialized a
	// single time throughout the calling program's lifetime.
	once sync.Once
)

// base are the base configuration options for the Temper API client.
type base struct {
	http    *http.Client
	baseURL string
}

// client is a Temper API client.
type client struct {
	base
	filter *filter
}

// Option contains all of the configuration options for the Temper API client.
type Option struct {
	// The base URL of the Temper instance, defaults to https://temperhq.com.
	BaseURL string

	// Features that are overridden in local development. Changes made here should
	// never be checked in, but just in case they are, the values here are
	// ignored when an API key is provided, preventing accidental overrides in
	// a production-like environment.
	TestModeOverrides map[string]struct{}
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

// Init initializes the Temper API client library using the given keys and
// optional configuration options.
func Init(publishableKey, secretKey string, opts ...*Option) {
	once.Do(func() {
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
		c = &client{base: *common}

		if err := c.fetchFilter(); err != nil {
			log.Printf("go-temper: failed to fetch and intialize filter: %s, retrying in 60 seconds, all checks will return false", err.Error())
			c.filter = &filter{}
		}
		go c.pollFilter()
	})
}

// fetchFilter gets the filter and rollout data from the Temper backend.
func (c *client) fetchFilter() error {
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

// TODO Refactor this and the other occasional backend checks to use `time.Ticker`.
func (c *client) pollFilter() {
	for {
		time.Sleep(60 * time.Second)

		if err := c.fetchFilter(); err != nil {
			log.Printf("go-temper: latest filter poll failed at %s due to error: %s", time.Now().String(), err.Error())
		}
	}
}

// Check looks up a single feature, returning true if it's enabled, and false
// otherwise.
func Check(feature string) bool {
	return c.filter.lookup([]byte(feature))
}

// Refactor runs both functions on the given RefactorArgs simultaneously,
// saving both results in Temper if they don't match. The return value is the
// result of the given RefactorArgs's `Old` function.
//
// If you need to return an error, use `RefactorErr`.
func Refactor[Args, Ret any](refactor *RefactorArgs[Args, Ret], args Args) Ret {
	return refactor.run(args)
}
