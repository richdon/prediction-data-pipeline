package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// httpClient is shared by all REST calls. The timeout bounds the whole
// request, including reading the body, so a hung call can't block the
// discovery loop forever.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// fetch sends a request with headers h and returns the response body.
// It returns an error for any status other than 200 OK, including up to 500
// bytes of the body, which holds Kalshi's error message.
func fetch(ctx context.Context, method, url string, h http.Header) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = h
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status %s: %s", resp.Status, b[:min(len(b), 500)])
	}
	return b, nil
}

// openEvents returns the open events for cfg.Series, with their markets
// nested in each event. The request is retried with backoff; decode errors
// are not, since the same body would fail again.
func openEvents(ctx context.Context, cfg Config) ([]Event, error) {
	u, err := url.Parse(cfg.BaseUrlRest + cfg.PathRest + "/events")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("series_ticker", cfg.Series)
	q.Set("status", "open")
	q.Set("with_nested_markets", "true")
	u.RawQuery = q.Encode()

	var r json.RawMessage
	err = retry(ctx, 3, time.Second, func() error {
		// re-sign per attempt: the timestamp goes stale between retries
		h := buildAuthHeaders(cfg.PrivateKeyPath, cfg.ApiKeyID, "GET", cfg.PathRest, "/events")
		var err error
		r, err = fetch(ctx, "GET", u.String(), h)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}

	var data EventsResponse
	if err := json.Unmarshal(r, &data); err != nil {
		return nil, fmt.Errorf("decode events: %w, body was: %s", err, r[:min(len(r), 500)])
	}
	return data.Events, nil
}
