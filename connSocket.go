package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type SubscriptionManager struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  int
	sids    map[string]int  // from the subscribe ack
	current map[string]bool // tickers we believe we're subscribed to
}

func (s *SubscriptionManager) Send(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

func (s *SubscriptionManager) NextCmdID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return s.nextID
}

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
	return nil
}

func (s *SubscriptionManager) ReadMessage(ctx context.Context, ec EventChannels) {
	defer close(ec.Tickers)
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			return
		}
		var e Envelope
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

func buildSubscriptionManager(cfg Config) (sm *SubscriptionManager, err error) {
	authHeaders := buildAuthHeaders(cfg.PrivateKeyPath, cfg.ApiKeyID, "GET", cfg.PathWs, "")
	url := cfg.BaseUrlWs+cfg.PathWs
	conn, _, err := websocket.DefaultDialer.Dial(url, authHeaders)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return
	}
	sm = &SubscriptionManager{
		conn:    conn,
		nextID:  1,
		sids:    make(map[string]int),
		current: make(map[string]bool),
	}
	return
}