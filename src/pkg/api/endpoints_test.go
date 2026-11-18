package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type seen struct {
	method, path string
	query        url.Values
	header       http.Header
	body         []byte
}

// serve replies with status and body to every request and records the last one.
func serve(t *testing.T, status int, body string) (*Client, *seen) {
	t.Helper()
	s := &seen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.method, s.path, s.query, s.header = r.Method, r.URL.Path, r.URL.Query(), r.Header.Clone()
		s.body, _ = io.ReadAll(r.Body)
		w.Header().Set("X-Asset-Filename", "a.txt")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, Key: "pmk_test", sleep: (&recorder{}).sleep}, s
}

func TestJSONEndpoints(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		reply      string
		call       func(c *Client) (status int, raw json.RawMessage, err error)
		method     string
		path       string
		query      url.Values
		body       string
		wantNoAuth bool
		check      func(t *testing.T, c *Client)
	}{
		{
			name: "score", reply: `{"id":"j1","status":"queued"}`, method: "POST", path: "/v1/score",
			body: `{"channel":"EMAIL","account":{"name":"Acme"},"variants":[{"text":"hi"}]}`,
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Score(ctx, &ScoreRequest{Channel: "EMAIL", Account: Account{Name: "Acme"}, Variants: []Touchpoints{{"text": "hi"}}}, 0)
				if err != nil {
					return 0, nil, err
				}
				if r.Value.ID != "j1" || r.Value.Status != "queued" {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "score wait", reply: `{"id":"j1"}`, method: "POST", path: "/v1/score", query: url.Values{"wait": {"25"}},
			body: `{"account":{"name":"Acme"},"variants":[{"text":"hi"}]}`,
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Score(ctx, &ScoreRequest{Account: Account{Name: "Acme"}, Variants: []Touchpoints{{"text": "hi"}}}, 25)
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "optimize", reply: `{"id":"j2"}`, method: "POST", path: "/v1/optimize", query: url.Values{"wait": {"50"}},
			body: `{"account":{"name":"Acme"},"variant":{"text":"hi"},"context":"c"}`,
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Optimize(ctx, &OptimizeRequest{Account: Account{Name: "Acme"}, Variant: Touchpoints{"text": "hi"}, Context: "c"}, 50)
				if err != nil {
					return 0, nil, err
				}
				if r.Value.ID != "j2" {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "optimize no wait", reply: `{}`, method: "POST", path: "/v1/optimize",
			body: `{"account":{"name":"A"},"variant":{"text":"x"}}`,
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Optimize(ctx, &OptimizeRequest{Account: Account{Name: "A"}, Variant: Touchpoints{"text": "x"}}, -1)
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "performance", reply: `{}`, method: "POST", path: "/v1/performance",
			body: `{"account":{"name":"A"},"variants":[{"text":"x"}]}`,
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Performance(ctx, &PerformanceRequest{Account: Account{Name: "A"}, Variants: []Touchpoints{{"text": "x"}}})
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "uplift", reply: `{}`, method: "POST", path: "/v1/uplift",
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Uplift(ctx, &UpliftRequest{Account: Account{Name: "A"}, Variant: Touchpoints{"text": "x"}})
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "job", reply: `{"id":"a/b c","status":"completed"}`, method: "GET", path: "/v1/jobs/a/b c",
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Job(ctx, "a/b c")
				if err != nil {
					return 0, nil, err
				}
				if !r.Value.Done() {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
			check: func(t *testing.T, c *Client) {},
		},
		{
			name: "catalogue", reply: `{}`, method: "GET", path: "/v1/catalogue",
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.Catalogue(ctx)
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "version", reply: `{"version":"1.2.3","environment":"stg"}`, method: "GET", path: "/version", wantNoAuth: true,
			call: func(c *Client) (int, json.RawMessage, error) {
				c.Key = ""
				r, err := c.Version(ctx)
				if err != nil {
					return 0, nil, err
				}
				if r.Value.Version != "1.2.3" || r.Value.Environment != "stg" {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "list assets default", reply: `{"assets":[{"id":"x"}],"nextCursor":"n"}`, method: "GET", path: "/v1/asset/list",
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.ListAssets(ctx, 0, "")
				if err != nil {
					return 0, nil, err
				}
				if len(r.Value.Assets) != 1 || r.Value.NextCursor != "n" {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "list assets limit cursor", reply: `{}`, method: "GET", path: "/v1/asset/list", query: url.Values{"limit": {"5"}, "cursor": {"c 1"}},
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.ListAssets(ctx, 5, "c 1")
				if err != nil {
					return 0, nil, err
				}
				return r.Status, r.Raw, nil
			},
		},
		{
			name: "asset link", reply: `{"id":"a1","url":"https://u"}`, method: "GET", path: "/v1/asset/retrieve", query: url.Values{"id": {"a1"}},
			call: func(c *Client) (int, json.RawMessage, error) {
				r, err := c.AssetLink(ctx, "a1")
				if err != nil {
					return 0, nil, err
				}
				if r.Value.URL != "https://u" {
					t.Errorf("value = %+v", r.Value)
				}
				return r.Status, r.Raw, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, s := serve(t, 200, tt.reply)
			status, raw, err := tt.call(c)
			if err != nil {
				t.Fatal(err)
			}
			if status != 200 || string(raw) != tt.reply {
				t.Errorf("status=%d raw=%s", status, raw)
			}
			if s.method != tt.method || s.path != tt.path {
				t.Errorf("request = %s %s", s.method, s.path)
			}
			if len(s.query) != len(tt.query) {
				t.Errorf("query = %v, want %v", s.query, tt.query)
			}
			for k, v := range tt.query {
				if s.query.Get(k) != v[0] {
					t.Errorf("query %s = %q, want %q", k, s.query.Get(k), v[0])
				}
			}
			if tt.body != "" {
				if string(s.body) != tt.body {
					t.Errorf("body = %s, want %s", s.body, tt.body)
				}
				if s.header.Get("Content-Type") != "application/json" {
					t.Errorf("content type = %q", s.header.Get("Content-Type"))
				}
			} else if tt.method == "GET" && len(s.body) != 0 {
				t.Errorf("GET body = %q", s.body)
			}
			if tt.wantNoAuth {
				if s.header.Get("Authorization") != "" {
					t.Errorf("Authorization sent: %q", s.header.Get("Authorization"))
				}
			} else if s.header.Get("Authorization") != "Bearer pmk_test" {
				t.Errorf("Authorization = %q", s.header.Get("Authorization"))
			}
		})
	}
}

func TestEndpointsRequireKey(t *testing.T) {
	c, _ := serve(t, 200, `{}`)
	c.Key = ""
	if _, err := c.Catalogue(context.Background()); err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Errorf("err = %v", err)
	}
}

func TestEndpointAPIError(t *testing.T) {
	c, _ := serve(t, 400, `{"error":{"code":"invalid_request","message":"bad","details":[{"field":"variants","reason":"required"}]}}`)
	_, err := c.Score(context.Background(), &ScoreRequest{}, 0)
	var e *Error
	if !errors.As(err, &e) || e.Status != 400 || len(e.Details) != 1 {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeError(t *testing.T) {
	c, _ := serve(t, 200, `not json`)
	r, err := c.Catalogue(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode /v1/catalogue response") || r != nil {
		t.Fatalf("r=%v err=%v", r, err)
	}
	c, _ = serve(t, 200, `[1,2]`)
	if _, err = c.Job(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Errorf("err = %v", err)
	}
}

func TestReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"a"`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Key: "k"}
	_, err := c.Catalogue(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read /v1/catalogue response") {
		t.Fatalf("err = %v", err)
	}
}

func TestCallMarshalError(t *testing.T) {
	c, _ := serve(t, 200, `{}`)
	_, err := call[Job](context.Background(), c, "POST", "/x", nil, make(chan int), false)
	if err == nil {
		t.Fatal("want marshal error")
	}
}

func TestCreateAsset(t *testing.T) {
	content := []byte("hello\x00binary\xff bytes")
	meta := &AssetMetadata{Filename: "a.bin", MimeType: "application/octet-stream", Length: int64(len(content)),
		ExpiresInDays: 3, Owner: "me", Attributes: map[string]string{"k": "v"}}
	c, s := serve(t, 201, `{"id":"as1","filename":"a.bin","size":19}`)
	r, err := c.CreateAsset(context.Background(), meta, strings.NewReader(string(content)))
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != 201 || r.Value.ID != "as1" {
		t.Errorf("resp = %+v", r)
	}
	if s.method != "POST" || s.path != "/v1/asset/create" || s.header.Get("Authorization") != "Bearer pmk_test" {
		t.Errorf("request = %s %s %v", s.method, s.path, s.header)
	}
	mt, params, err := mime.ParseMediaType(s.header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" {
		t.Fatalf("content type = %q %v", s.header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(strings.NewReader(string(s.body)), params["boundary"])

	p1, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if p1.FormName() != "metadata" {
		t.Errorf("first part = %q, want metadata", p1.FormName())
	}
	if ct := p1.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("metadata Content-Type = %q", ct)
	}
	mb, _ := io.ReadAll(p1)
	var got AssetMetadata
	if err := json.Unmarshal(mb, &got); err != nil {
		t.Fatal(err)
	}
	if got.Filename != "a.bin" || got.Length != int64(len(content)) || got.ExpiresInDays != 3 || got.Owner != "me" || got.Attributes["k"] != "v" {
		t.Errorf("metadata = %+v", got)
	}

	p2, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if p2.FormName() != "file" {
		t.Errorf("second part = %q, want file", p2.FormName())
	}
	if ct := p2.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("file Content-Type = %q", ct)
	}
	fb, _ := io.ReadAll(p2)
	if string(fb) != string(content) {
		t.Errorf("file bytes = %q", fb)
	}
	if _, err := mr.NextPart(); err != io.EOF {
		t.Errorf("extra part: %v", err)
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

func TestCreateAssetErrors(t *testing.T) {
	t.Run("file read error", func(t *testing.T) {
		c, s := serve(t, 201, `{}`)
		_, err := c.CreateAsset(context.Background(), &AssetMetadata{Filename: "f.txt"}, failReader{})
		if err == nil || !strings.Contains(err.Error(), "read f.txt: disk gone") {
			t.Fatalf("err = %v", err)
		}
		if s.method != "" {
			t.Error("request must not be sent")
		}
	})
	t.Run("api error", func(t *testing.T) {
		c, _ := serve(t, 400, `{"error":{"code":"length_mismatch","message":"length differs"}}`)
		_, err := c.CreateAsset(context.Background(), &AssetMetadata{Filename: "f"}, strings.NewReader("x"))
		var e *Error
		if !errors.As(err, &e) || e.Code != "length_mismatch" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("retries resend multipart body", func(t *testing.T) {
		var bodies [][]byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, b)
			if len(bodies) == 1 {
				w.WriteHeader(503)
				return
			}
			_, _ = w.Write([]byte(`{"id":"ok"}`))
		}))
		defer srv.Close()
		c := &Client{BaseURL: srv.URL, Key: "k", sleep: (&recorder{}).sleep}
		r, err := c.CreateAsset(context.Background(), &AssetMetadata{Filename: "f"}, strings.NewReader("data"))
		if err != nil || r.Value.ID != "ok" {
			t.Fatalf("r=%v err=%v", r, err)
		}
		if len(bodies) != 2 || string(bodies[0]) != string(bodies[1]) || !strings.Contains(string(bodies[1]), "data") {
			t.Errorf("bodies differ or lack data: %d", len(bodies))
		}
	})
}

func TestDownloadAsset(t *testing.T) {
	c, s := serve(t, 200, "raw-bytes")
	resp, err := c.DownloadAsset(context.Background(), "a 1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "raw-bytes" || resp.Header.Get("X-Asset-Filename") != "a.txt" {
		t.Errorf("body=%q headers=%v", b, resp.Header)
	}
	if s.method != "GET" || s.path != "/v1/asset/retrieve" || s.query.Get("stream") != "true" || s.query.Get("id") != "a 1" {
		t.Errorf("request = %s %s %v", s.method, s.path, s.query)
	}
	if s.header.Get("Authorization") != "Bearer pmk_test" {
		t.Errorf("Authorization = %q", s.header.Get("Authorization"))
	}

	t.Run("error", func(t *testing.T) {
		c, _ := serve(t, 404, `{"error":{"code":"not_found","message":"no such asset"}}`)
		resp, err := c.DownloadAsset(context.Background(), "x")
		var e *Error
		if resp != nil || !errors.As(err, &e) || e.Code != "not_found" {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
	})
}
