package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func fetch(url string) {

	var data any
	resp, err := http.Get(url)
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: %v\n", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: reading %s: %v\n", url, err)
	}
	json.Unmarshal(b, &data)
	out, _ := json.MarshalIndent(data, "", "    ")
	fmt.Println(string(out))
}
func main() {
	fetch("https://external-api.kalshi.com/trade-api/v2/markets?series_ticker=KXHIGHNY&status=open")
}
