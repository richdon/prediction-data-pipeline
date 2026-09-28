package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func fetch(url string) *http.Response {
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

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Decode the private key remove the -----BEGIN PRIVATE KEY----- parts
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	// Get the key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	// Use reflection to validate the return key is the required type because its an interface
	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 private key")
	}
	return privKey, nil
}

func signRequest(key ed25519.PrivateKey, method, path string) (string, string) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	path = strings.Split(path, "?")[0]
	msg := ts + method + path
	sig := ed25519.Sign(key, []byte(msg))
	return base64.StdEncoding.EncodeToString(sig), ts
}

func fetchAuthenticated(key ed25519.PrivateKey, apiKeyId, method, url, path, endpoint string) ([]byte, error) {
	sig, ts := signRequest(key, method, path+endpoint)
	req, _ := http.NewRequest(method, url+path+endpoint, nil)
	req.Header.Set("KALSHI-ACCESS-KEY", apiKeyId)
	req.Header.Set("KALSHI-ACCESS-TIMESTAMP", ts)
	req.Header.Set("KALSHI-ACCESS-SIGNATURE", sig)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte(""), err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Got bad status code: %d, status: %s", resp.StatusCode, resp.Status)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return b, nil
}

func doFetchAuthenticated(endpoint string) []byte {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	env := os.Getenv("ENV")
	apiKeyID := os.Getenv(env + "_API_KEY_ID")
	privKeyPath := os.Getenv(env + "_PRIVATE_KEY_PATH")
	key, err := loadPrivateKey(privKeyPath)
	if err != nil {
		log.Fatalf("could not get private key %s", err)
	}
	baseUrl := os.Getenv(env + "_BASE_URL")
	path := os.Getenv(env + "_PATH")
	b, err := fetchAuthenticated(key, apiKeyID, baseUrl, "GET", path, endpoint)
	if err != nil {
		log.Fatalf("could not get balance data %s", err)
	}
	return b
}

func connWebSocket(method string) (*websocket.Conn, error) {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	env := os.Getenv("ENV")
	apiKeyID := os.Getenv(env + "_API_KEY_ID")
	privKeyPath := os.Getenv(env + "_PRIVATE_KEY_PATH")
	key, err := loadPrivateKey(privKeyPath)
	if err != nil {
		return nil, err
	}
	path := os.Getenv(env + "_WS_PATH")
	sig, ts := signRequest(key, method, path)
	headers := http.Header{}
	headers.Set("KALSHI-ACCESS-KEY", apiKeyID)
	headers.Set("KALSHI-ACCESS-SIGNATURE", sig)
	headers.Set("KALSHI-ACCESS-TIMESTAMP", ts)
	url := os.Getenv(env + "_WS_BASE_URL")
	conn, _, err := websocket.DefaultDialer.Dial(url+path, headers)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func subscribe(conn *websocket.Conn, channels, tickers []string) error {
	msg := SubscribeMsg{
		ID:  1,
		Cmd: "subscribe",
		Params: Params{
			Channels: channels,
			Tickers:  tickers,
		},
	}
	return conn.WriteJSON(msg)
}

func readConn(conn *websocket.Conn, out chan<- TickerMessage) {
	defer close(out)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			return
		}
		var event TickerMessage
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Println("unmarshal error:", err)
			continue
		}
		out <- event
	}
}

func main() {

	conn, err := connWebSocket("GET")

	if err != nil {
		log.Panicln(err)
	}
	defer conn.Close()

	if err := subscribe(conn, []string{"ticker"}, []string{}); err != nil {
		log.Panicln(err)
	}
	tickerChan := make(chan TickerMessage, 1024)

	go readConn(conn, tickerChan)

	for m := range tickerChan {
		fmt.Printf("%s bid=%.2f ask=%.2f\n", m.Msg.MarketTicker, m.Msg.YesBid, m.Msg.YesAsk)
	}

	// s := Series{NycWeather: "KXHIGHNY"}

	// weatherSeriesUrls := SeriesUrls{
	// 	SeriesInfo:  fmt.Sprintf("https://external-api.kalshi.com/trade-api/v2/series/%s", s.NycWeather),
	// 	OpenMarkets: fmt.Sprintf("https://external-api.kalshi.com/trade-api/v2/markets?series_ticker=%s&status=open", s.NycWeather),
	// }

	// weatherSeriesInfo, err := seriesInfoData(weatherSeriesUrls.SeriesInfo)
	// if err != nil {
	// 	log.Panicln(err)
	// }
	// log.Printf("series: %s\ninfo:%v\n", s, weatherSeriesInfo)
	// weatherMarketInfo, err := marketsData(weatherSeriesUrls.OpenMarkets)
	// if err != nil {
	// 	log.Panicln(err)
	// }

	// for _, market := range weatherMarketInfo {
	// 	fmt.Fprintf(os.Stdout, "Title: %s, Ticker: %s\n", market.Title, market.Ticker)
	// }
	// b := doFetchAuthenticated("/markets")
	// //bal := Balance{}
	// var m MarketsResponse
	// json.Unmarshal(b, &m)
	// //fmt.Fprintf(os.Stdout, "balance data: %s\n", bal.BalanceDollars)
	// d, _ := json.MarshalIndent(m, "", "  ")
	// fmt.Fprintf(os.Stdout, "the data: %s", d)

}
