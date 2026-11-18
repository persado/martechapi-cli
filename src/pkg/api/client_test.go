package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recorder struct{ delays []time.Duration }

func (r *recorder) sleep(_ context.Context, d time.Duration) error {
	r.delays = append(r.delays, d)
	return nil
}

func newTestClient(srv *httptest.Server, rec *recorder) *Client {
	c := &Client{BaseURL: srv.URL, Key: "pmk_test"}
	if rec != nil {
		c.sleep = rec.sleep
	}
	return c
}

func TestDoHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	t.Run("all set", func(t *testing.T) {
		c := newTestClient(srv, nil)
		c.UserAgent, c.Language = "ua/1", "el"
		resp, err := c.Do(context.Background(), Request{Method: "POST", Path: "/x", Body: []byte("{}"), ContentType: "application/json"})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if got.Get("Authorization") != "Bearer pmk_test" || got.Get("User-Agent") != "ua/1" ||
			got.Get("Accept-Language") != "el" || got.Get("Content-Type") != "application/json" {
			t.Errorf("headers = %v", got)
		}
	})
	t.Run("optional absent", func(t *testing.T) {
		c := newTestClient(srv, nil)
		resp, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if got.Get("Accept-Language") != "" || got.Get("Content-Type") != "" {
			t.Errorf("headers = %v", got)
		}
		if strings.HasPrefix(got.Get("User-Agent"), "ua/") {
			t.Errorf("user agent = %q", got.Get("User-Agent"))
		}
	})
	t.Run("no auth", func(t *testing.T) {
		c := &Client{BaseURL: srv.URL}
		resp, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x", NoAuth: true})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if got.Get("Authorization") != "" {
			t.Errorf("Authorization = %q", got.Get("Authorization"))
		}
	})
}

func TestDoMissingKey(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL}
	_, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"})
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Fatalf("err = %v", err)
	}
	if hits != 0 {
		t.Errorf("server was called %d times", hits)
	}
}

func TestClientURL(t *testing.T) {
	tests := []struct {
		name, base, path string
		query            map[string][]string
		want             string
		wantErr          bool
	}{
		{"default", "", "/v1/x", nil, DefaultBaseURL + "/v1/x", false},
		{"no scheme", "api.example.com/api", "/v1/x", nil, "https://api.example.com/api/v1/x", false},
		{"trailing slash", "http://h:1/api/", "/v1/x", nil, "http://h:1/api/v1/x", false},
		{"query", "http://h", "/v1/x", map[string][]string{"a": {"1"}, "b": {"x y"}}, "http://h/v1/x?a=1&b=x+y", false},
		{"invalid", "http://bad host", "/v1/x", nil, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{BaseURL: tt.base}
			got, err := c.url(tt.path, tt.query)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), "invalid base URL") {
					t.Errorf("err = %v", err)
				}
				return
			}
			if got != tt.want {
				t.Errorf("url = %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("Do reports invalid base", func(t *testing.T) {
		c := &Client{BaseURL: "http://bad host", Key: "k"}
		if _, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"}); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestDoRequestErrors(t *testing.T) {
	t.Run("bad method", func(t *testing.T) {
		c := &Client{BaseURL: "http://127.0.0.1:1", Key: "k"}
		if _, err := c.Do(context.Background(), Request{Method: "BAD METHOD", Path: "/x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		base := srv.URL
		srv.Close()
		c := &Client{BaseURL: base, Key: "k"}
		if _, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("custom http client", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()
		var used bool
		c := newTestClient(srv, nil)
		c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			used = true
			return http.DefaultTransport.RoundTrip(r)
		})}
		resp, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if !used {
			t.Error("custom HTTP client not used")
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestErrorParsing(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		header  map[string]string
		body    string
		want    Error
		wantStr []string
	}{
		{
			name:   "envelope with details",
			status: 400, body: `{"error":{"code":"invalid_request","message":"bad input","requestId":"r1","details":[{"field":"variants","reason":"required"},{"field":"channel","reason":"unknown"}]}}`,
			want:    Error{Status: 400, Code: "invalid_request", Message: "bad input", RequestID: "r1"},
			wantStr: []string{"bad input (HTTP 400, invalid_request)", "\n  variants: required", "\n  channel: unknown", "request id: r1"},
		},
		{
			name: "categories", status: 422,
			body:    `{"error":{"code":"pii","message":"personal data","categories":["email","phone"]}}`,
			want:    Error{Status: 422, Code: "pii", Message: "personal data"},
			wantStr: []string{"personal data found: email, phone"},
		},
		{
			name: "request id from header", status: 500, header: map[string]string{"X-Request-Id": "hdr-9"},
			body:    `{"error":{"code":"internal","message":"boom"}}`,
			want:    Error{Status: 500, Code: "internal", Message: "boom", RequestID: "hdr-9"},
			wantStr: []string{"request id: hdr-9"},
		},
		{
			name: "body id wins over header", status: 500, header: map[string]string{"X-Request-Id": "hdr"},
			body:    `{"error":{"message":"m","requestId":"body"}}`,
			want:    Error{Status: 500, Message: "m", RequestID: "body"},
			wantStr: []string{"m (HTTP 500)", "request id: body"},
		},
		{
			name: "non-json body", status: 502, body: "  Bad Gateway from proxy \n",
			want:    Error{Status: 502, Message: "Bad Gateway from proxy"},
			wantStr: []string{"Bad Gateway from proxy (HTTP 502)"},
		},
		{
			name: "json without error key", status: 400, body: `{"foo":1}`,
			want: Error{Status: 400, Message: `{"foo":1}`},
		},
		{
			name: "empty body", status: 404, body: "",
			want:    Error{Status: 404, Message: "Not Found"},
			wantStr: []string{"Not Found (HTTP 404)"},
		},
		{
			name: "envelope without message", status: 403, body: `{"error":{"code":"forbidden"}}`,
			want: Error{Status: 403, Code: "forbidden", Message: `{"error":{"code":"forbidden"}}`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := newTestClient(srv, &recorder{})
			resp, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"})
			if resp != nil {
				t.Fatal("want nil response on error")
			}
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("err = %T %v", err, err)
			}
			if e.Status != tt.want.Status || e.Code != tt.want.Code || e.Message != tt.want.Message || e.RequestID != tt.want.RequestID {
				t.Errorf("got %+v, want %+v", *e, tt.want)
			}
			for _, s := range tt.wantStr {
				if !strings.Contains(e.Error(), s) {
					t.Errorf("Error() = %q, missing %q", e.Error(), s)
				}
			}
		})
	}

	t.Run("details and categories decoded", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":{"message":"m","details":[{"field":"f","reason":"r"}],"categories":["c"]}}`))
		}))
		defer srv.Close()
		_, err := newTestClient(srv, nil).Do(context.Background(), Request{Method: "GET", Path: "/x"})
		var e *Error
		if !errors.As(err, &e) {
			t.Fatal(err)
		}
		if len(e.Details) != 1 || e.Details[0] != (ErrorDetail{Field: "f", Reason: "r"}) || len(e.Categories) != 1 || e.Categories[0] != "c" {
			t.Errorf("got %+v", *e)
		}
	})
}

func TestRetry(t *testing.T) {
	tests := []struct {
		name       string
		responses  []response
		wantDelays []time.Duration
		wantCalls  int
		wantErr    bool
		wantCode   string
	}{
		{
			name:       "429 rate_limited then ok, default backoff",
			responses:  []response{{429, nil, `{"error":{"code":"rate_limited","message":"slow"}}`}, {200, nil, `{}`}},
			wantDelays: []time.Duration{time.Second}, wantCalls: 2,
		},
		{
			name:       "503 then ok",
			responses:  []response{{503, nil, ``}, {200, nil, `{}`}},
			wantDelays: []time.Duration{time.Second}, wantCalls: 2,
		},
		{
			name: "Retry-After header",
			responses: []response{
				{429, map[string]string{"Retry-After": "7"}, `{"error":{"code":"rate_limited","message":"x","retryAfterSeconds":3}}`},
				{200, nil, `{}`}},
			wantDelays: []time.Duration{7 * time.Second}, wantCalls: 2,
		},
		{
			name:       "Retry-After zero",
			responses:  []response{{503, map[string]string{"Retry-After": "0"}, ``}, {200, nil, `{}`}},
			wantDelays: []time.Duration{0}, wantCalls: 2,
		},
		{
			name:       "retryAfterSeconds body",
			responses:  []response{{429, nil, `{"error":{"code":"rate_limited","message":"x","retryAfterSeconds":5}}`}, {200, nil, `{}`}},
			wantDelays: []time.Duration{5 * time.Second}, wantCalls: 2,
		},
		{
			name:       "negative Retry-After falls back to body",
			responses:  []response{{429, map[string]string{"Retry-After": "-4"}, `{"error":{"code":"rate_limited","message":"x","retryAfterSeconds":6}}`}, {200, nil, `{}`}},
			wantDelays: []time.Duration{6 * time.Second}, wantCalls: 2,
		},
		{
			name:       "non-numeric Retry-After falls back to backoff",
			responses:  []response{{503, map[string]string{"Retry-After": "Wed, 21 Oct 2026 07:28:00 GMT"}, ``}, {200, nil, `{}`}},
			wantDelays: []time.Duration{time.Second}, wantCalls: 2,
		},
		{
			name:       "delay capped at 30s",
			responses:  []response{{503, map[string]string{"Retry-After": "600"}, ``}, {200, nil, `{}`}},
			wantDelays: []time.Duration{30 * time.Second}, wantCalls: 2,
		},
		{
			name:       "exponential backoff",
			responses:  []response{{503, nil, ``}, {503, nil, ``}, {503, nil, ``}, {200, nil, `{}`}},
			wantDelays: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, wantCalls: 4,
		},
		{
			name:       "cap: gives up after maxRetries",
			responses:  []response{{503, nil, `{"error":{"code":"unavailable","message":"down"}}`}},
			wantDelays: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, wantCalls: 4,
			wantErr: true, wantCode: "unavailable",
		},
		{
			name:       "quota_exhausted not retried",
			responses:  []response{{429, map[string]string{"Retry-After": "1"}, `{"error":{"code":"quota_exhausted","message":"no quota"}}`}},
			wantDelays: nil, wantCalls: 1, wantErr: true, wantCode: "quota_exhausted",
		},
		{
			name:       "429 without code not retried",
			responses:  []response{{429, nil, `slow down`}},
			wantDelays: nil, wantCalls: 1, wantErr: true,
		},
		{
			name:       "400 not retried",
			responses:  []response{{400, nil, `{"error":{"code":"invalid_request","message":"bad"}}`}},
			wantDelays: nil, wantCalls: 1, wantErr: true, wantCode: "invalid_request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var bodies []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b := make([]byte, 16)
				n, _ := r.Body.Read(b)
				bodies = append(bodies, string(b[:n]))
				i := min(calls, len(tt.responses)-1)
				calls++
				rs := tt.responses[i]
				for k, v := range rs.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(rs.status)
				_, _ = w.Write([]byte(rs.body))
			}))
			defer srv.Close()
			rec := &recorder{}
			c := newTestClient(srv, rec)
			resp, err := c.Do(context.Background(), Request{Method: "POST", Path: "/x", Body: []byte("payload"), ContentType: "text/plain"})
			if resp != nil {
				_ = resp.Body.Close()
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				var e *Error
				if !errors.As(err, &e) {
					t.Fatalf("err = %T", err)
				}
				if tt.wantCode != "" && e.Code != tt.wantCode {
					t.Errorf("code = %q", e.Code)
				}
			}
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if len(rec.delays) != len(tt.wantDelays) {
				t.Fatalf("delays = %v, want %v", rec.delays, tt.wantDelays)
			}
			for i := range rec.delays {
				if rec.delays[i] != tt.wantDelays[i] {
					t.Errorf("delays = %v, want %v", rec.delays, tt.wantDelays)
				}
			}
			for i, b := range bodies {
				if b != "payload" {
					t.Errorf("attempt %d body = %q, want body resent unchanged", i, b)
				}
			}
		})
	}
}

type response struct {
	status int
	header map[string]string
	body   string
}

func TestRetrySleepError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	c := newTestClient(srv, nil)
	c.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	_, err := c.Do(context.Background(), Request{Method: "GET", Path: "/x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryContextCancelDuringSleep(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(503)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{BaseURL: srv.URL, Key: "k"} // real sleepCtx
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	start := time.Now()
	_, err := c.Do(ctx, Request{Method: "GET", Path: "/x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("did not return promptly on cancel")
	}
	if calls != 1 {
		t.Errorf("calls = %d", calls)
	}
}

func TestSleepCtx(t *testing.T) {
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestErrorStringMinimal(t *testing.T) {
	e := &Error{Status: 500, Message: "x"}
	if got := e.Error(); got != "x (HTTP 500)" {
		t.Errorf("got %q", got)
	}
}
