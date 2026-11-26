package booksearch

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	requestTimeout   = 5 * time.Second
	maxResponseBytes = 1 << 20 // 1 MiB
	userAgent        = "boardoflife/1.0"
)

// NewHTTPClient returns the client used to reach the catalogues: short
// timeout, and no redirect following (the hosts are fixed, and a redirect
// could otherwise carry an API key elsewhere).
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// getJSON GETs base?query and decodes a bounded JSON body into dst. Returned
// errors mention only the provider name, the status code or the low-level
// cause, never the URL.
func getJSON(ctx context.Context, client *http.Client, name, base string, query url.Values, dst any) error {
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("%s: invalid base URL", name)
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("%s: build request", name)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error // its text embeds the URL; keep only the cause
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("%s: request failed: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: upstream status %d", name, resp.StatusCode)
	}
	if err := json.UnmarshalRead(io.LimitReader(resp.Body, maxResponseBytes), dst); err != nil {
		return fmt.Errorf("%s: decode response (%T)", name, err)
	}
	return nil
}
