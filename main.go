package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	cfg := loadConfig("KXBTCD", "BRTI", "hourly")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
				fmt.Printf("%s bid=%.2f ask=%.2f\n", t.MarketTicker, t.YesBid, t.YesAsk)
			case it := <-ec.Index:
				fmt.Printf("%s value=%.2f avg60s=%.2f (n=%d)\n", it.IndexID, it.Value, it.Avg60s, it.Avg60sWindow)
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

// marketTickers returns the market tickers of the first open event in
// cfg.Series whose cadence matches cfg.Cadence.
func marketTickers(ctx context.Context, cfg Config) (tickers []string, err error) {
	events, err := openEvents(ctx, cfg)
	if err != nil {
		return []string{}, err
	}
	for _, event := range events {
		if event.ProductMetadata.Cadence == cfg.Cadence {
			for _, market := range event.Markets {
				if market.Status == "active" {
					tickers = append(tickers, market.Ticker)
				}
			}
		}
	}
	if len(tickers) > 0 {
		return tickers, nil

	}
	return tickers, fmt.Errorf("selected cadence: %s did match any in series", cfg.Cadence)
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
	}
}
