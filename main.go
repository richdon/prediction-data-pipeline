package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)


func fetch(url string) *http.Response{
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	return resp
	
}

func seriesInfoData(url string) (SeriesInfo, error) {
	r := fetch(url)
	b, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return SeriesInfo{}, err
	}
	var data = SeriesInfoResponse{}
	json.Unmarshal(b, &data)
	return data.Series, nil
}

func marketsData(url string) ([]Market, error) {
	r := fetch(url)
	b, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return []Market{}, err
	}
	var data = MarketsResponse{}
	json.Unmarshal(b, &data)
	return data.Markets, nil
}

func main() {
	s := Series{NycWeather: "KXHIGHNY"}

	weatherSeriesUrls := SeriesUrls{
		SeriesInfo: fmt.Sprintf("https://external-api.kalshi.com/trade-api/v2/series/%s", s.NycWeather),
		OpenMarkets: fmt.Sprintf("https://external-api.kalshi.com/trade-api/v2/markets?series_ticker=%s&status=open", s.NycWeather),
	} 
	
	weatherSeriesInfo, err := seriesInfoData(weatherSeriesUrls.SeriesInfo)
	if err != nil {
		log.Panicln(err)
	}
	log.Printf("series: %s\ninfo:%v\n", s, weatherSeriesInfo)
	weatherMarketInfo, err := marketsData(weatherSeriesUrls.OpenMarkets)
	if err != nil {
		log.Panicln(err)
	}
	
	for _, market := range weatherMarketInfo {
		fmt.Fprintf(os.Stdout, "Title: %s, Ticker: %s\n", market.Title, market.Ticker)
	}
}

