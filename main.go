package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	cfg := loadConfig("KXBTCD", "BRTI", "hourly")
	if _, ok := cadenceRank[cfg.Cadence]; !ok {
		log.Fatalf("unknown cadence %q: want hourly, daily or weekly", cfg.Cadence)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	w := newWriter(cfg)

	defer func() {
		if err := w.Close(); err != nil {
			log.Println("kafka close: ", err)
		}
	}()

	for {
		connCtx, cancel := context.WithCancel(ctx)
		sm, err := buildSubscriptionManager(connCtx, cfg)
		if err != nil {
			log.Println(err)
			continue
		}
		go discoveryLoop(connCtx, cfg, sm)
		// closing the conn is what unblocks ReadMessage on shutdown
		go func() {
			<-connCtx.Done()
			sm.conn.Close()
		}()

		ec := EventChannels{
			Tickers: make(chan TickerData, 1024),
			Index:   make(chan IndexTick, 256),
			Errors:  make(chan ReadError),
		}

		go sm.ReadMessage(connCtx, ec)
	inner:
		for {
			select {
			case err := <-ec.Errors:
				log.Println("connection err: ", err)
				cancel()
				sm.conn.Close()
				break inner
			case t := <-ec.Tickers:
				if err := publishTicker(ctx, w, t, cfg); err != nil {
					log.Println("error publishing ticker data: ", err)
				}
			case it := <-ec.Index:
				if err := publishIndex(ctx, w, it, cfg); err != nil {
					log.Println("error publishing index tick: ", err)
				}
			case <-connCtx.Done():
				log.Println("connection closed, exiting...")
				return
			}
		}
	}
}

// discoveryLoop reconciles the ticker subscription with the current open
// event straight away, then again 30 seconds after every hour, until ctx is
// done. A failed discovery or reconcile is logged and the existing
// subscription is kept until the next attempt.
func discoveryLoop(ctx context.Context, cfg Config, sm *SubscriptionManager) {
	sm.Send(Message{
		ID:  sm.NextCmdID(),
		Cmd: "subscribe",
		Params: Params{
			Channels: []string{"cfbenchmarks_value"},
			IndexIDs: []string{cfg.IndexID},
		},
	})
	for {
		// on failure keep the existing subscription and retry next hour
		if tickers, err := marketTickers(ctx, cfg); err != nil {
			log.Println("discovery failed, keeping current subscription:", err)
		} else if err := sm.Reconcile(tickers); err != nil {
			log.Println("reconcile failed:", err)
		}
		wait := time.Until(nextTopOfHour(time.Now()).Add(30 * time.Second))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			log.Println("starting new subscription loop")
		}
	}
}

// cadenceRank orders cadences by window length. When a longer event settles at
// the same time as a shorter one would, Kalshi lists only the longer event: no
// hourly event exists for the hour ending 17:00 ET because the daily settles
// then, and likewise for daily vs weekly on Fridays.
var cadenceRank = map[string]int{"hourly": 1, "daily": 2, "weekly": 3}

// marketTickers returns the active market tickers of the open event(s) in
// cfg.Series that settle soonest at cfg.Cadence. Events of a longer cadence
// also count, since they stand in for the configured cadence when their
// settlement times coincide (see cadenceRank). Events sharing the earliest
// strike date are all included. Events whose strike date has passed are
// skipped, since a closed event can still be listed briefly before it settles.
func marketTickers(ctx context.Context, cfg Config) ([]string, error) {
	events, err := openEvents(ctx, cfg)
	if err != nil {
		return nil, err
	}

	minRank := cadenceRank[cfg.Cadence]
	eligible := func(e Event) bool {
		return cadenceRank[e.ProductMetadata.Cadence] >= minRank && e.StrikeDate.After(time.Now())
	}

	var next time.Time
	for _, event := range events {
		if eligible(event) && (next.IsZero() || event.StrikeDate.Before(next)) {
			next = event.StrikeDate
		}
	}
	if next.IsZero() {
		return nil, fmt.Errorf("no open %s event at %s cadence or longer with a future strike date", cfg.Series, cfg.Cadence)
	}

	var tickers []string
	for _, event := range events {
		if !eligible(event) || !event.StrikeDate.Equal(next) {
			continue
		}
		n := 0
		for _, market := range event.Markets {
			if market.Status == "active" {
				tickers = append(tickers, market.Ticker)
				n++
			}
		}
		log.Printf("selected event %s (%s, settles %s): %d active markets",
			event.EventTicker, event.ProductMetadata.Cadence, event.StrikeDate.Local().Format("15:04 Jan 2"), n)
	}
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no active markets in %s events settling %s", cfg.Series, next.Local().Format("15:04 Jan 2"))
	}
	return tickers, nil
}

// nextTopOfHour returns the start of the hour after t.
func nextTopOfHour(t time.Time) time.Time {
	return t.Truncate(time.Hour).Add(time.Hour)
}

// loadConfig builds a Config for series and cadence from .env, reading
// variables prefixed with the value of ENV (for example PROD_API_KEY_ID).
// It exits if .env cannot be loaded.
func loadConfig(series, indexID, cadence string) Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	env := os.Getenv("ENV")

	baseUrlWs := os.Getenv(env + "_WS_BASE_URL")
	pathWs := os.Getenv(env + "_WS_PATH")

	baseUrlRest := os.Getenv(env + "_REST_BASE_URL")
	pathRest := os.Getenv(env + "_REST_PATH")

	apiKeyID := os.Getenv(env + "_API_KEY_ID")
	privKeyPath := os.Getenv(env + "_PRIVATE_KEY_PATH")
	brokers := strings.Split(os.Getenv("BROKERS"), ",")
	return Config{
		Env:            env,
		ApiKeyID:       apiKeyID,
		PrivateKeyPath: privKeyPath,
		BaseUrlWs:      baseUrlWs,
		PathWs:         pathWs,
		BaseUrlRest:    baseUrlRest,
		PathRest:       pathRest,
		Series:         series,
		IndexID:        indexID,
		Cadence:        cadence,
		Brokers:        brokers,
		TopicTicker:    os.Getenv("TOPIC_TICKER"),
		TopicIndex:     os.Getenv("TOPIC_INDEX"),
	}
}
