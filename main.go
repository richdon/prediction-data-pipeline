package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	cfg := loadConfig()

	tickers, err := marketTickers(cfg, "KXBTCD", "hourly")
	if err != nil {
		log.Panicln(err)
	}

	authHeaders := buildAuthHeaders(cfg.PrivateKeyPath, cfg.ApiKeyID, "GET", cfg.PathWs, "")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, err := connWebSocket("GET", cfg.BaseUrlWs+cfg.PathWs, authHeaders)

	if err != nil {
		log.Panicln(err)
	}
	// closing the conn is what unblocks ReadMessage on shutdown
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	if err := subscribe(conn, []string{"ticker"}, tickers); err != nil {
		log.Panicln(err)
	}
	ec := EventChannels{Tickers: make(chan TickerData, 1024)}

	go readConn(ctx, conn, ec)

	for t := range ec.Tickers {
		fmt.Printf("%s bid=%.2f ask=%.2f\n", t.MarketTicker, t.YesBid, t.YesAsk)
	}
}

func marketTickers(cfg Config, seriesTicker, cadence string) (tickers []string,err error){
	events, err := openEvents(cfg, seriesTicker)
	if err != nil {
		return []string{}, err
	}
	for _, event := range events{
		if event.ProductMetadata.Cadence == cadence {
			selectedEvent := event
			for _, market := range selectedEvent.Markets {
				tickers = append(tickers, market.Ticker)
			}
			return
		}
	}
	return tickers, fmt.Errorf("selected cadence: %s did match any in series", cadence)
}

func loadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	env := os.Getenv("ENV")

	baseUrlWs := os.Getenv(env+"_WS_BASE_URL")
	pathWs := os.Getenv(env + "_WS_PATH")

	baseUrlRest := os.Getenv(env+"_REST_BASE_URL")
	pathRest := os.Getenv(env + "_REST_PATH")
	
	apiKeyID := os.Getenv(env + "_API_KEY_ID")
	privKeyPath := os.Getenv(env + "_PRIVATE_KEY_PATH")
	return Config{
		Env: env,
		ApiKeyID: apiKeyID,
		PrivateKeyPath: privKeyPath,
		BaseUrlWs: baseUrlWs,
		PathWs: pathWs,
		BaseUrlRest: baseUrlRest,
		PathRest: pathRest,
	}
}