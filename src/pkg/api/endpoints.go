package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

// Response is a decoded answer with the raw JSON body, so callers can print
// exactly what the server sent.
type Response[T any] struct {
	Status int
	Raw    json.RawMessage
	Value  T
}

func call[T any](ctx context.Context, c *Client, method, path string, query url.Values, in any, noAuth bool) (*Response[T], error) {
	r := Request{Method: method, Path: path, Query: query, NoAuth: noAuth}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		r.Body, r.ContentType = b, "application/json"
	}
	return do[T](ctx, c, r)
}

func do[T any](ctx context.Context, c *Client, r Request) (*Response[T], error) {
	resp, err := c.Do(ctx, r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", r.Path, err)
	}
	out := &Response[T]{Status: resp.StatusCode, Raw: raw}
	if err := json.Unmarshal(raw, &out.Value); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", r.Path, err)
	}
	return out, nil
}

func waitQuery(wait int) url.Values {
	if wait <= 0 {
		return nil
	}
	return url.Values{"wait": {strconv.Itoa(wait)}}
}

// Score starts a score run. wait (0 to 50 s) holds the request open for the job.
func (c *Client) Score(ctx context.Context, r *ScoreRequest, wait int) (*Response[Job], error) {
	return call[Job](ctx, c, http.MethodPost, "/v1/score", waitQuery(wait), r, false)
}

// Optimize starts an optimize run. wait (0 to 50 s) holds the request open for the job.
func (c *Client) Optimize(ctx context.Context, r *OptimizeRequest, wait int) (*Response[Job], error) {
	return call[Job](ctx, c, http.MethodPost, "/v1/optimize", waitQuery(wait), r, false)
}

// Performance predicts the performance of 1 to 10 variants.
func (c *Client) Performance(ctx context.Context, r *PerformanceRequest) (*Response[PerformanceResponse], error) {
	return call[PerformanceResponse](ctx, c, http.MethodPost, "/v1/performance", nil, r, false)
}

// Uplift projects the optimization potential and uplift of one variant.
func (c *Client) Uplift(ctx context.Context, r *UpliftRequest) (*Response[UpliftResponse], error) {
	return call[UpliftResponse](ctx, c, http.MethodPost, "/v1/uplift", nil, r, false)
}

// Job reads a job.
func (c *Client) Job(ctx context.Context, id string) (*Response[Job], error) {
	return call[Job](ctx, c, http.MethodGet, "/v1/jobs/"+url.PathEscape(id), nil, nil, false)
}

// Catalogue reads the channels, display channels, industries and limits.
func (c *Client) Catalogue(ctx context.Context) (*Response[Catalogue], error) {
	return call[Catalogue](ctx, c, http.MethodGet, "/v1/catalogue", nil, nil, false)
}

// Version reads the server's build and spec version. It needs no key.
func (c *Client) Version(ctx context.Context) (*Response[VersionInfo], error) {
	return call[VersionInfo](ctx, c, http.MethodGet, "/version", nil, nil, true)
}

// ListAssets reads one page of assets, newest first. limit 0 uses the server default.
func (c *Client) ListAssets(ctx context.Context, limit int, cursor string) (*Response[AssetList], error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return call[AssetList](ctx, c, http.MethodGet, "/v1/asset/list", q, nil, false)
}

// CreateAsset uploads file, which must hold exactly m.Length bytes.
func (c *Client) CreateAsset(ctx context.Context, m *AssetMetadata, file io.Reader) (*Response[Asset], error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	meta, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="metadata"`)
	h.Set("Content-Type", "application/json")
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(meta); err != nil {
		return nil, err
	}
	h = textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="file"`)
	h.Set("Content-Type", "application/octet-stream")
	if part, err = w.CreatePart(h); err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("read %s: %w", m.Filename, err)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return do[Asset](ctx, c, Request{Method: http.MethodPost, Path: "/v1/asset/create",
		Body: body.Bytes(), ContentType: w.FormDataContentType()})
}

// AssetLink returns a presigned URL for an asset, valid for one hour.
func (c *Client) AssetLink(ctx context.Context, id string) (*Response[AssetURL], error) {
	return call[AssetURL](ctx, c, http.MethodGet, "/v1/asset/retrieve", url.Values{"id": {id}}, nil, false)
}

// DownloadAsset streams an asset's bytes. The caller closes the body. The
// response headers carry X-Asset-Filename and the other X-Asset-* fields.
func (c *Client) DownloadAsset(ctx context.Context, id string) (*http.Response, error) {
	return c.Do(ctx, Request{Method: http.MethodGet, Path: "/v1/asset/retrieve",
		Query: url.Values{"id": {id}, "stream": {"true"}}})
}
