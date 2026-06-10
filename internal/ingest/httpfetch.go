package ingest

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// defaultMaxFetchBytes caps a single attachment download. Federal RFP packages
// are large but not unbounded; this guards against a pathological response
// exhausting memory. Override via NewHTTPFetcher.
const defaultMaxFetchBytes = 64 << 20 // 64 MiB

// HTTPFetcher is the production Fetcher: it downloads an attachment over HTTP(S).
//
// Re-downloading is avoided upstream by the ingest stage's SHA-256 idempotency
// check (an unchanged document is fetched but not re-uploaded). When composed
// behind a caching/rate-limiting transport (e.g. the one guarding the SAM.gov
// quota), pass that http.Client in via NewHTTPFetcher.
type HTTPFetcher struct {
	client   *http.Client
	maxBytes int64
}

// NewHTTPFetcher builds an HTTPFetcher. A nil client gets a default client with a
// sane timeout; a non-positive maxBytes gets defaultMaxFetchBytes.
func NewHTTPFetcher(client *http.Client, maxBytes int64) *HTTPFetcher {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxFetchBytes
	}
	return &HTTPFetcher{client: client, maxBytes: maxBytes}
}

// Fetch downloads rawURL and returns its bytes, content type, and a filename
// derived from the Content-Disposition header or the URL path.
func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) (data []byte, contentType, filename string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, "", "", fmt.Errorf("httpfetch: build request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("httpfetch: get %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("httpfetch: get %s: unexpected status %s", rawURL, resp.Status)
	}

	// Read at most maxBytes+1 so we can detect an over-limit body.
	data, err = io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("httpfetch: read %s: %w", rawURL, err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, "", "", fmt.Errorf("httpfetch: %s exceeds %d byte limit", rawURL, f.maxBytes)
	}

	contentType = resp.Header.Get("Content-Type")
	filename = filenameFromResponse(resp, rawURL)
	return data, contentType, filename, nil
}

// filenameFromResponse prefers the Content-Disposition filename and falls back to
// the last path segment of the URL.
func filenameFromResponse(resp *http.Response, rawURL string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if name := strings.TrimSpace(params["filename"]); name != "" {
				return path.Base(name)
			}
		}
	}
	if u, err := url.Parse(rawURL); err == nil {
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	return ""
}
