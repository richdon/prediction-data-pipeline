package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func connWebSocket(method, url string, headers http.Header) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, headers)
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

func readConn(ctx context.Context, conn *websocket.Conn, ec EventChannels) {
	defer close(ec.Tickers)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			return
		}
		var e Envelope
		if err := json.Unmarshal(msg, &e); err != nil {
			log.Println("unmarshal envelope error:", err)
			continue
		}

		if e.Type != "ticker" {
			log.Println("skipping unhandled event type: ", e.Type)
			continue
		}

		var t TickerData
		if err := json.Unmarshal(e.Msg, &t); err != nil {
			log.Println("unmarshal ticker data error:", err)
			continue
		}
		select {
		case ec.Tickers <- t:
			continue
		case <-ctx.Done():
			return
		}
	}
}
