package main

import (
	"encoding/json"
	"time"
)

type Series struct {
	NycWeather string
}

type SeriesUrls struct {
	SeriesInfo  string
	OpenMarkets string
}

type EventUrls struct {
	Events    string
	Orderbook string
}

type SeriesInfoResponse struct {
	Series SeriesInfo `json:"series"`
}
type SeriesInfo struct {
	Category      string `json:"category"`
	FeeMultiplier int    `json:"fee_multiplier"`
	FeeType       string `json:"Quadratic"`
	Frequnecy     string `json:"daily"`
	Ticker        string `json:"ticker"`
	Title         string `json:"title"`
	LastUpdated   string `json:"last_updated_ts"`
}

type Market struct {
	Ticker      string `json:"ticker"`
	EventTicker string `json:"event_ticker"`
	Title       string `json:"title"`
	Status      string `json:"status"` // "active", "closed", etc.

	// Strike / contract definition
	StrikeType  string  `json:"strike_type"` // "between", "greater", etc.
	FloorStrike float64 `json:"floor_strike"`
	CapStrike   float64 `json:"cap_strike"`

	// Pricing (yes side)
	YesBid float64 `json:"yes_bid_dollars,string"`
	YesAsk float64 `json:"yes_ask_dollars,string"`

	// Pricing (no side)
	NoBid float64 `json:"no_bid_dollars,string"`
	NoAsk float64 `json:"no_ask_dollars,string"`

	LastPrice float64 `json:"last_price_dollars,string"`

	// Activity / liquidity
	Volume24h    float64 `json:"volume_24h_fp,string"`
	Volume       float64 `json:"volume_fp,string"`
	OpenInterest float64 `json:"open_interest_fp,string"`
	Liquidity    float64 `json:"liquidity_dollars,string"`

	// Timing
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
}

type MarketsResponse struct {
	Markets []Market `json:"markets"`
}

type BalanceResponse struct {
	Data Balance
}

type Balance struct {
	BalanceDollars string  `json:"balance_dollars"`
	BalanceData    float64 `json:"balance"`
}

type SubscribeMsg struct {
	ID     int    `json:"id"`
	Cmd    string `json:"cmd"`
	Params Params `json:"params"`
}

type Params struct {
	Channels []string `json:"channels"`
	Tickers  []string `json:"market_tickers,omitempty"`
}

type TickerData struct {
	MarketID     string `json:"market_id"`
	MarketTicker string `json:"market_ticker"`

	// Prices (sent as strings)
	Price  float64 `json:"price_dollars,string"`
	YesBid float64 `json:"yes_bid_dollars,string"`
	YesAsk float64 `json:"yes_ask_dollars,string"`

	// Sizes / volume (sent as strings)
	YesBidSize    float64 `json:"yes_bid_size_fp,string"`
	YesAskSize    float64 `json:"yes_ask_size_fp,string"`
	LastTradeSize float64 `json:"last_trade_size_fp,string"`
	Volume        float64 `json:"volume_fp,string"`
	OpenInterest  float64 `json:"open_interest_fp,string"`

	// Sent as real JSON numbers, so no ,string tag
	DollarVolume       float64 `json:"dollar_volume"`
	DollarOpenInterest float64 `json:"dollar_open_interest"`

	// Timing
	Time time.Time `json:"time"`
	TS   int64     `json:"ts"`    // unix seconds
	TSMs int64     `json:"ts_ms"` // unix milliseconds
}

type Envelope struct {
	Type string          `json:"type"`
	SID  int             `json:"sid"`
	Msg  json.RawMessage `json:"msg"`
}

type EventChannels struct {
	Tickers chan TickerData
}

type Config struct {
	Env        string
	ApiKeyID   string
	PrivateKeyPath string
	BaseUrlRest string
	BaseUrlWs string
	PathRest string
	PathWs string
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
