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
	endpoint := os.Args[1]
	c := loadConfig()
	authHeaders := buildAuthHeaders(c.PrivateKeyPath, c.ApiKeyID, "GET", c.PathWs, endpoint)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	conn, err := connWebSocket("GET", c.BaseUrlWs+c.PathWs, authHeaders)

	if err != nil {
		log.Panicln(err)
	}
	// closing the conn is what unblocks ReadMessage on shutdown
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	if err := subscribe(conn, []string{"ticker"}, []string{}); err != nil {
		log.Panicln(err)
	}
	ec := EventChannels{Tickers: make(chan TickerData, 1024)}

	go readConn(ctx, conn, ec)

	for t := range ec.Tickers {
		fmt.Printf("%s bid=%.2f ask=%.2f\n", t.MarketTicker, t.YesBid, t.YesAsk)
	}
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