package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// fetch sends a request with headers h and returns the response body.
// It returns an error for any status other than 200 OK.
func fetch(method, url string, h http.Header) (b json.RawMessage, err error){
	req, _ := http.NewRequest(method, url, nil)
	req.Header = h
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		return b, fmt.Errorf("Got bad status code: %d, status: %s", resp.StatusCode, resp.Status)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	return json.RawMessage(b), nil
}

// openEvents returns the open events for cfg.Series, with their markets
// nested in each event.
func openEvents(cfg Config) (events []Event, err error) {
	u, err := url.Parse(cfg.BaseUrlRest + cfg.PathRest + "/events")
    if err != nil {
        return 
    }
	h := buildAuthHeaders(cfg.PrivateKeyPath, cfg.ApiKeyID, "GET", cfg.PathRest, "/events")
	q := u.Query()
    q.Set("series_ticker", cfg.Series)
    q.Set("status", "open")
    q.Set("with_nested_markets", "true")
    u.RawQuery = q.Encode()
	r, err := fetch("GET", u.String(), h) 
	if err != nil {
		return
	}
	var data EventsResponse
	if err = json.Unmarshal(r, &data); err != nil {
		return nil, fmt.Errorf("decode events: %w, body was: %s", err, r[:min(len(r), 500)])
	}
	return data.Events, nil
}