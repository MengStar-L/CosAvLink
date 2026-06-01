// Package flaresolverr provides an HTTP client for the FlareSolverr proxy
// service, which solves Cloudflare challenges automatically.
//
// It replaces the previous go-rod based browser approach, allowing the
// application to run in headless Linux environments without a local Chrome
// installation.
package flaresolverr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrBlocked indicates the target served a Cloudflare block/challenge page
// instead of real content.
var ErrBlocked = errors.New("blocked by FlareSolverr")

// Options configures the FlareSolverr client.
type Options struct {
	// URL is the FlareSolverr API endpoint (default http://localhost:8191/v1).
	URL string
	// MaxParallel bounds concurrent requests to FlareSolverr (default 2).
	MaxParallel int
	// MaxTimeout is the per-request timeout in milliseconds for FlareSolverr
	// to solve a challenge (default 60000).
	MaxTimeout int
}

// Client performs HTTP requests through the FlareSolverr proxy service.
type Client struct {
	url        string
	httpClient *http.Client
	sem        chan struct{}
	maxTimeout int
}

// New returns a Client. Call Close when done (no-op but satisfies interface).
func New(opts Options) *Client {
	if opts.URL == "" {
		opts.URL = "http://localhost:8191/v1"
	}
	if opts.MaxParallel < 1 {
		opts.MaxParallel = 2
	}
	if opts.MaxTimeout < 1 {
		opts.MaxTimeout = 60000
	}
	return &Client{
		url:        opts.URL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		sem:        make(chan struct{}, opts.MaxParallel),
		maxTimeout: opts.MaxTimeout,
	}
}

// flaresolverrRequest is the JSON body sent to FlareSolverr.
type flaresolverrRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
}

// flaresolverrResponse is the JSON body returned by FlareSolverr.
type flaresolverrResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Solution struct {
		URL      string            `json:"url"`
		Status   int               `json:"status"`
		Response string            `json:"response"`
		Cookies  []flaresolverrCookie `json:"cookies"`
	} `json:"solution"`
}

type flaresolverrCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// Result holds the HTML and metadata returned by FlareSolverr.
type Result struct {
	// HTML is the full page HTML after solving any challenges.
	HTML string
	// StatusCode is the HTTP status code of the target page.
	StatusCode int
	// FinalURL is the URL after any redirects.
	FinalURL string
	// Cookies are the cookies set by the target (including cf_clearance).
	Cookies []flaresolverrCookie
}

// Get fetches the given URL through FlareSolverr, automatically solving any
// Cloudflare challenges. It is concurrency-limited by Options.MaxParallel.
func (c *Client) Get(ctx context.Context, url string) (*Result, error) {
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-c.sem }()

	reqBody := flaresolverrRequest{
		Cmd:        "request.get",
		URL:        url,
		MaxTimeout: c.maxTimeout,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("flaresolverr request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flaresolverr returned HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	var fsResp flaresolverrResponse
	if err := json.Unmarshal(respBody, &fsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if fsResp.Status != "ok" {
		return nil, fmt.Errorf("flaresolverr error: %s", fsResp.Message)
	}

	return &Result{
		HTML:       fsResp.Solution.Response,
		StatusCode: fsResp.Solution.Status,
		FinalURL:   fsResp.Solution.URL,
		Cookies:    fsResp.Solution.Cookies,
	}, nil
}

// Close is a no-op (FlareSolverr is an external service).
func (c *Client) Close() error { return nil }

// LooksBlocked reports whether the given page title/HTML is a Cloudflare
// block or "checking your browser" interstitial rather than real content.
func LooksBlocked(title, html string) bool {
	t := strings.ToLower(title)
	for _, marker := range []string{"just a moment", "attention required", "you have been blocked"} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	h := strings.ToLower(html)
	for _, marker := range []string{
		"/cdn-cgi/styles/cf.errors",
		"cf-error-details",
		"checking your browser before accessing",
		"sorry, you have been blocked",
	} {
		if strings.Contains(h, marker) {
			return true
		}
	}
	return false
}
