package main

import (
	"encoding/json"
	"net/http"
	"io"
	"fmt"
)

func fetch(method, url string, headers http.Header ) (json.RawMessage, error){
	req, _ := http.NewRequest(method, url, nil)
	req.Header = headers
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Got bad status code: %d, status: %s", resp.StatusCode, resp.Status)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return json.RawMessage(b), nil
}

