package main

import (
	"encoding/json"
	"time"
)

// Market is one strike within an event, as returned by the REST API.
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

// Message is a WebSocket subscribe command.
type Message struct {
	ID     int    `json:"id"`
	Cmd    string `json:"cmd"`
	Params Params `json:"params"`
}

// Params holds the channels and market tickers for a subscribe command.
type Params struct {
	Channels []string `json:"channels"`
	Tickers  []string `json:"market_tickers,omitempty"`
	IndexIDs []string `json:"index_ids,omitempty"`
}

// TickerData is the payload of a "ticker" message: top-of-book quotes, sizes
// and activity for one market.
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

// Envelope is the outer shape of every WebSocket message. Msg is decoded
// separately once Type is known.
type Envelope struct {
	Type string          `json:"type"`
	SID  int             `json:"sid"`
	Seq  int             `json:"seq"` // per-subscription; a jump means missed messages
	Msg  json.RawMessage `json:"msg"`
}

// EventsResponse is the body of GET /events.
type EventsResponse struct {
	Cursor string  `json:"cursor"`
	Events []Event `json:"events"`
}

// Event is one betting window in a series, with a single deadline shared by
// all of its markets.
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

// ProductMetadata holds an event's cadence.
type ProductMetadata struct {
	Cadence string `json:"cadence"`
}

// SettlementSource names the reference an event settles against.
type SettlementSource struct {
	Name string `json:"name"` // "CF Benchmarks"
	URL  string `json:"url"`
}

// EventChannels carries decoded WebSocket data from ReadMessage to its
// consumers.
type EventChannels struct {
	Tickers chan TickerData
	Index   chan IndexTick
	Errors  chan ReadError
}

type ReadError struct {
	Error error
}

// Config holds the connection settings loaded from the environment and the
// series and cadence to track.
type Config struct {
	Env            string
	ApiKeyID       string
	PrivateKeyPath string
	BaseUrlRest    string
	BaseUrlWs      string
	PathRest       string
	PathWs         string
	Series         string
	IndexID        string
	Cadence        string
}

// SubscribedMsg is the payload of a "subscribed" ack. The ack carries the sid
// inside msg, unlike data messages where it is top-level.
type SubscribedMsg struct {
	Channel string `json:"channel"`
	SID     int    `json:"sid"`
}

// UpdateSubMsg is an update_subscription command.
type UpdateSubMsg struct {
	ID     int          `json:"id"`
	Cmd    string       `json:"cmd"`
	Params UpdateParams `json:"params"`
}

// UpdateParams holds the sids, market tickers and action for an
// update_subscription command.
type UpdateParams struct {
	SIDs    []int    `json:"sids"`
	Tickers []string `json:"market_tickers"`
	Action  string   `json:"action"` // "add_markets" | "delete_markets"
}

type CFBenchmarksMsg struct {
	IndexID    string         `json:"index_id"`
	ReceivedAt int64          `json:"received_at"` // unix ms
	Data       string         `json:"data"`        // raw CF frame, decode into CFFrame
	Avg60s     WindowedAvg    `json:"avg_60s_data"`
	Avg15m     *WindowedAvg   `json:"last_60s_windowed_average_15min,omitempty"` // optional
}

type CFFrame struct {
	ID    string  `json:"id"`
	Time  int64   `json:"time"` // unix ms, CF's timestamp
	Value float64 `json:"value,string"`
}

type WindowedAvg struct {
	Value       float64 `json:"value,string"`
	WindowSize  int     `json:"window_size"`
	StartTsMs   int64   `json:"window_start_ts_ms"`
	EndTsMsExcl int64   `json:"window_end_ts_exclusive"`
}

// IndexTick is one reference index update, flattened from a cfbenchmarks_value
// message into the fields the model uses.
type IndexTick struct {
	IndexID      string  // e.g. "BRTI"
	Value        float64 // index value, from the inner CF data frame
	SourceTsMs   int64   // CF's timestamp: when the value was true (event time)
	ReceivedAtMs int64   // when Kalshi received it: same clock as ticker data
	Avg60s       float64 // trailing 60-second average; the settlement quantity in the final minute
	Avg60sWindow int     // ticks in that average; well under 60 means missing ticks
	Seq          int     // envelope seq, for gap detection
}