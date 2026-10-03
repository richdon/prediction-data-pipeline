package main

import (
	"encoding/json"
	"time"
)

type Market struct {
	Ticker      string `json:"ticker"`
	EventTicker string `json:"event_ticker"`
	Title       string `json:"title"`
	SubTitle    string `json:"subtitle"` // "$83,400 or above"
	Status      string `json:"status"`

	// Strike definition
	StrikeType  string   `json:"strike_type"` // "greater", "between", ...
	FloorStrike float64  `json:"floor_strike"`
	CapStrike   *float64 `json:"cap_strike,omitempty"` // nil for "greater"

	// Top of book
	YesBid     float64 `json:"yes_bid_dollars,string"`
	YesAsk     float64 `json:"yes_ask_dollars,string"`
	YesBidSize float64 `json:"yes_bid_size_fp,string"`
	YesAskSize float64 `json:"yes_ask_size_fp,string"`

	LastPrice float64 `json:"last_price_dollars,string"`

	// Activity
	Volume       float64 `json:"volume_fp,string"`
	Volume24h    float64 `json:"volume_24h_fp,string"`
	OpenInterest float64 `json:"open_interest_fp,string"`

	// Timing
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
}

type Message struct {
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

type EventsResponse struct {
	Cursor string  `json:"cursor"`
	Events []Event `json:"events"`
}

type Event struct {
	EventTicker  string `json:"event_ticker"`
	SeriesTicker string `json:"series_ticker"`
	Title        string `json:"title"`     // "BTC price on Sep 29, 2026 at 11pm EDT?"
	SubTitle     string `json:"sub_title"` // "On Sep 29, 2026 at 11pm EDT"

	// Which window this event is — "hourly", "daily", "weekly".
	// The only reliable way to tell them apart; the ticker won't.
	ProductMetadata ProductMetadata `json:"product_metadata"`

	// false means the strikes are a cumulative "or above" ladder,
	// not exclusive buckets.
	MutuallyExclusive bool `json:"mutually_exclusive"`

	StrikeDate        time.Time          `json:"strike_date"`
	SettlementSources []SettlementSource `json:"settlement_sources"`

	Markets []Market `json:"markets"` // populated by with_nested_markets=true
}

type ProductMetadata struct {
	Cadence string `json:"cadence"`
}

type SettlementSource struct {
	Name string `json:"name"` // "CF Benchmarks"
	URL  string `json:"url"`
}

type EventChannels struct {
	Tickers chan TickerData
}

type Config struct {
	Env            string
	ApiKeyID       string
	PrivateKeyPath string
	BaseUrlRest    string
	BaseUrlWs      string
	PathRest       string
	PathWs         string
	Series         string
	Cadence        string
}

// The ack carries the sid inside msg, unlike data messages where it is top-level.
type SubscribedMsg struct {
	Channel string `json:"channel"`
	SID     int    `json:"sid"`
}

type UpdateSubMsg struct {
	ID     int          `json:"id"`
	Cmd    string       `json:"cmd"`
	Params UpdateParams `json:"params"`
}

type UpdateParams struct {
	SIDs    []int    `json:"sids"`
	Tickers []string `json:"market_tickers"`
	Action  string   `json:"action"` // "add_markets" | "delete_markets"
}