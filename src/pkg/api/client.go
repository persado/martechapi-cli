// Package api is a client for the Persado Martech API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the API region used when none is configured.
const DefaultBaseURL = "https://api.persado.com/api"

// maxRetries is how many times a request rejected with 429 rate_limited or 503 is resent.
const maxRetries = 3

// maxRetryDelay caps the Retry-After the client honours between attempts.
const maxRetryDelay = 30 * time.Second

// Client calls the Martech API. The zero HTTP uses http.DefaultClient.
type Client struct {
	BaseURL   string
	Key       string
	UserAgent string
	Language  string
	HTTP      *http.Client
	// sleep waits between retries; tests replace it.
	sleep func(context.Context, time.Duration) error
}

// Request is one API call. Body is resent unchanged on a retry.
type Request struct {
	Method      string
	Path        string
	Query       url.Values
	Body        []byte
	ContentType string
	NoAuth      bool
}

// Error is an API error response.
type Error struct {
	Status     int
	Code       string        `json:"code"`
	Message    string        `json:"message"`
	RequestID  string        `json:"requestId"`
	Details    []ErrorDetail `json:"details"`
	Categories []string      `json:"categories"`
	RetryAfter int           `json:"retryAfterSeconds"`
}

// ErrorDetail names one invalid field.
type ErrorDetail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (HTTP %d", e.Message, e.Status)
	if e.Code != "" {
		fmt.Fprintf(&b, ", %s", e.Code)
	}
	b.WriteString(")")
	for _, d := range e.Details {
		fmt.Fprintf(&b, "\n  %s: %s", d.Field, d.Reason)
	}
	if len(e.Categories) > 0 {
		fmt.Fprintf(&b, "\n  personal data found: %s", strings.Join(e.Categories, ", "))
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, "\n  request id: %s", e.RequestID)
	}
	return b.String()
}

// Do sends r and returns the response for a 2xx status. Any other status is
// returned as *Error. The caller closes the response body.
func (c *Client) Do(ctx context.Context, r Request) (*http.Response, error) {
	if !r.NoAuth && c.Key == "" {
		return nil, errors.New("no API key: set PERSADO_API_KEY, or pass --api-key or --api-key-file")
	}
	u, err := c.url(r.Path, r.Query)
	if err != nil {
		return nil, err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	sleep := c.sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, r.Method, u, bytes.NewReader(r.Body))
		if err != nil {
			return nil, err
		}
		if r.Body != nil {
			req.Header.Set("Content-Type", r.ContentType)
		}
		if !r.NoAuth {
			req.Header.Set("Authorization", "Bearer "+c.Key)
		}
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}
		if c.Language != "" {
			req.Header.Set("Accept-Language", c.Language)
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		apiErr := readError(resp)
		if attempt >= maxRetries || !retryable(apiErr) {
			return nil, apiErr
		}
		if err := sleep(ctx, retryDelay(resp, apiErr, attempt)); err != nil {
			return nil, err
		}
	}
}

func (c *Client) url(path string, query url.Values) (string, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + path)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", c.BaseURL, err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func readError(resp *http.Response) *Error {
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		Error *Error `json:"error"`
	}
	e := &Error{}
	if json.Unmarshal(body, &env) == nil && env.Error != nil {
		e = env.Error
	}
	e.Status = resp.StatusCode
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
		if e.Message == "" {
			e.Message = http.StatusText(resp.StatusCode)
		}
	}
	if e.RequestID == "" {
		e.RequestID = resp.Header.Get("X-Request-Id")
	}
	return e
}

func retryable(e *Error) bool {
	return (e.Status == http.StatusTooManyRequests && e.Code == "rate_limited") ||
		e.Status == http.StatusServiceUnavailable
}

func retryDelay(resp *http.Response, e *Error, attempt int) time.Duration {
	d := time.Duration(1<<attempt) * time.Second
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s >= 0 {
		d = time.Duration(s) * time.Second
	} else if e.RetryAfter > 0 {
		d = time.Duration(e.RetryAfter) * time.Second
	}
	return min(d, maxRetryDelay)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
