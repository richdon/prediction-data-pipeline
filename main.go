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
	cfg := loadConfig("KXBTCD", "hourly")
	sm, err := buildSubscriptionManager(cfg)
	if err != nil {
		log.Panicln(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go discoveryLoop(ctx, cfg, sm)
	// closing the conn is what unblocks ReadMessage on shutdown
	go func() {
		<-ctx.Done()
		sm.conn.Close()
	}()

	ec := EventChannels{Tickers: make(chan TickerData, 1024)}

	go sm.ReadMessage(ctx, ec)

	for t := range ec.Tickers {
		fmt.Printf("%s bid=%.2f ask=%.2f\n", t.MarketTicker, t.YesBid, t.YesAsk)
	}
}

// discoveryLoop reconciles the ticker subscription with the current open
// event straight away, then again 30 seconds after every hour, until ctx is
// done. A failed discovery or reconcile is logged and the existing
// subscription is kept until the next attempt.
func discoveryLoop(ctx context.Context, cfg Config, sm *SubscriptionManager) {
	for {
		// on failure keep the existing subscription and retry next hour
		if tickers, err := marketTickers(cfg); err != nil {
			log.Println("discovery failed, keeping current subscription:", err)
		} else if err := sm.Reconcile(tickers); err != nil {
			log.Println("reconcile failed:", err)
		}
		wait := time.Until(nextTopOfHour(time.Now()).Add(30*time.Second))
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
func marketTickers(cfg Config) (tickers []string, err error) {
	events, err := openEvents(cfg)
	if err != nil {
		return []string{}, err
	}
	for _, event := range events {
		if event.ProductMetadata.Cadence == cfg.Cadence {
			selectedEvent := event
			for _, market := range selectedEvent.Markets {
				tickers = append(tickers, market.Ticker)
			}
			return
		}
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
func loadConfig(series, cadence string) Config {
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
		Cadence:        cadence,
	}
}
