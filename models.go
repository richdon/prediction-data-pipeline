package main

import "time"

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
