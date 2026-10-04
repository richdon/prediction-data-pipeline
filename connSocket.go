package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SubscriptionManager owns the Kalshi WebSocket connection and the client's view
// of its subscriptions. All writes go through Send, and mu guards nextID, sids
// and current.
type SubscriptionManager struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  int
	sids    map[string]int  // from the subscribe ack
	current map[string]bool // tickers we believe we're subscribed to
}

// Send writes v to the connection as JSON. It holds s.mu so that writes from
// different goroutines can't interleave on the socket.
func (s *SubscriptionManager) Send(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

// NextCmdID returns a new command id. Kalshi echoes it on the response to
// that command.
func (s *SubscriptionManager) NextCmdID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return s.nextID
}

// SubscribeMessage sends a subscribe command for tickers on each of channels.
// The sid arrives later in a "subscribed" ack, handled by ReadMessage.
func (s *SubscriptionManager) SubscribeMessage(channels, tickers []string) error {
	msg := Message{
		ID:  s.NextCmdID(),
		Cmd: "subscribe",
		Params: Params{
			Channels: channels,
			Tickers:  tickers,
		},
	}
	return s.Send(msg)
}

// UpdateMessage sends an update_subscription command that applies action
// ("add_markets" or "delete_markets") to tickers on subscription sid.
// It sends nothing if tickers is empty.
func (s *SubscriptionManager) UpdateMessage(sid int, tickers []string, action string) error {
	if len(tickers) == 0 {
		return nil // nothing to do
	}
	return s.Send(UpdateSubMsg{
		ID:  s.NextCmdID(),
		Cmd: "update_subscription",
		Params: UpdateParams{
			SIDs:    []int{sid},
			Tickers: tickers,
			Action:  action,
		},
	})
}

// RecordSID stores the sid the server assigned to channel.
func (s *SubscriptionManager) RecordSID(sid int, channel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sids[channel] = sid
}

// AddTickers records tickers as subscribed. Call only after the command was sent.
func (s *SubscriptionManager) AddTickers(tickers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range tickers {
		s.current[t] = true
	}
}

// RemoveTickers records tickers as unsubscribed. Call only after the command was sent.
func (s *SubscriptionManager) RemoveTickers(tickers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range tickers {
		delete(s.current, t)
	}
}

// ClearTickers forgets every recorded ticker so the next Reconcile re-adds them.
func (s *SubscriptionManager) ClearTickers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.current)
}

// Reconcile diffs the wanted tickers against current and sends the changes.
// current is only updated after a send succeeds, so a failed diff is retried
// on the next call.
func (s *SubscriptionManager) Reconcile(tickers []string) error {
	s.mu.Lock()
	sid, ok := s.sids["ticker"]
	want := make(map[string]bool, len(tickers))
	for _, t := range tickers {
		want[t] = true
	}
	var toAdd, toRemove []string

	for t := range want {
		if !s.current[t] {
			toAdd = append(toAdd, t)
		}
	}
	for t := range s.current {
		if !want[t] {
			toRemove = append(toRemove, t)
		}
	}
	s.mu.Unlock()
	if !ok {
		// no sid yet means this is going to be the first subscribe
		if err := s.SubscribeMessage([]string{"ticker"}, tickers); err != nil {
			return err
		}
		s.AddTickers(tickers)
		return nil
	}
	if err := s.UpdateMessage(sid, toAdd, "add_markets"); err != nil {
		return err
	}
	s.AddTickers(toAdd)
	if err := s.UpdateMessage(sid, toRemove, "delete_markets"); err != nil {
		return err
	}
	s.RemoveTickers(toRemove)
	log.Printf("reconcile: want=%d +%d -%d", len(tickers), len(toAdd), len(toRemove))
	return nil
}

// ReadMessage reads from the connection until it errors or ctx is done,
// forwarding ticker data to ec.Tickers and recording sids from subscribe acks.
// It closes ec.Tickers on return. Closing the connection is what unblocks it
// on shutdown.
func (s *SubscriptionManager) ReadMessage(ctx context.Context, ec EventChannels) {
	defer close(ec.Tickers)
	defer close(ec.Index)
	defer close(ec.Errors)
	lastSeq := make(map[int]int) // sid -> last seq seen, for gap detection
	for {
		var e Envelope
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			select {
			case ec.Errors <- ReadError{err}:
				return
			case <-ctx.Done():
				return
			}
		}
		if err := json.Unmarshal(msg, &e); err != nil {
			log.Println("unmarshal envelope error:", err)
			continue
		}
		switch e.Type {
		case "ticker":
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
		case "subscribed":
			var sm SubscribedMsg
			if err := json.Unmarshal(e.Msg, &sm); err != nil {
				log.Println("subscribed unmarshal:", err)
				continue
			}
			s.RecordSID(sm.SID, sm.Channel)
		case "cfbenchmarks_value":
			var cfbMsg CFBenchmarksMsg
			if err := json.Unmarshal(e.Msg, &cfbMsg); err != nil {
				log.Println("unmarshal error:", err)
				continue
			}
			var cff CFFrame
			if err := json.Unmarshal([]byte(cfbMsg.Data), &cff); err != nil {
				log.Println("unmarshal error:", err)
				continue
			}
			if last, ok := lastSeq[e.SID]; ok && e.Seq != last+1 {
				log.Printf("%s: seq gap, expected %d got %d", cfbMsg.IndexID, last+1, e.Seq)
			}
			lastSeq[e.SID] = e.Seq
			it := IndexTick{
				IndexID:      cfbMsg.IndexID,
				Value:        cff.Value,
				SourceTsMs:   cff.Time,
				ReceivedAtMs: cfbMsg.ReceivedAt,
				Avg60s:       cfbMsg.Avg60s.Value,
				Avg60sWindow: cfbMsg.Avg60s.WindowSize,
				Seq:          e.Seq,
			}
			select {
			case ec.Index <- it:
				continue
			case <-ctx.Done():
				return
			}
		case "error":
			// a rejected command means current may not match the server, so
			// forget it and let the next reconcile re-add everything it wants
			log.Println("server error:", string(e.Msg))
			s.ClearTickers()
			continue
		}

		if e.Type != "ticker" {
			//log.Println("skipping unhandled event type: ", e.Type)
			var d any
			json.Unmarshal(e.Msg, &d)
			log.Println(e.Msg)
			continue
		}
	}
}

// buildSubscriptionManager opens an authenticated WebSocket connection to
// Kalshi and returns a SubscriptionManager for it with no subscriptions yet.
func buildSubscriptionManager(ctx context.Context, cfg Config) (*SubscriptionManager, error) {
	url := cfg.BaseUrlWs + cfg.PathWs

	var conn *websocket.Conn
	connect := func() error {
		// re-sign per attempt: the timestamp goes stale between retries
		h := buildAuthHeaders(cfg.PrivateKeyPath, cfg.ApiKeyID, "GET", cfg.PathWs, "")

		c, resp, err := websocket.DefaultDialer.Dial(url, h)
		if err != nil {
			if resp != nil {
				return fmt.Errorf("dial %s: %w (status %d)", url, err, resp.StatusCode)
			}
			return fmt.Errorf("dial %s: %w", url, err)
		}
		conn = c
		return nil
	}

	if err := retry(ctx, 10, 2*time.Second, connect); err != nil {
		return nil, fmt.Errorf("connecting websocket: %w", err)
	}

	return &SubscriptionManager{
		conn:    conn,
		nextID:  1,
		sids:    make(map[string]int),
		current: make(map[string]bool),
	}, nil
}
