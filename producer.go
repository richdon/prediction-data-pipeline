package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

func newWriter(cfg Config) *kafka.Writer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        true,
		BatchTimeout: 50 * time.Millisecond,
		Compression:  kafka.Zstd,
		Completion: func(m []kafka.Message, err error) {
			if err != nil {
				log.Println("kafka:", len(m), err)
			}
		},
	}
	return w
}

func publishTicker(ctx context.Context, w *kafka.Writer, t TickerData, cfg Config) error {
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}
		err = w.WriteMessages(ctx, kafka.Message{
			Key: []byte(t.MarketTicker),
			Topic: cfg.TopicTicker,
			Value: data,
			Time: time.UnixMilli(t.TSMs),
		})
		if err != nil {
			return err
		}
		fmt.Printf("Published ticker message: %s bid=%.2f ask=%.2f\n", t.MarketTicker, t.YesBid, t.YesAsk)
		return nil
}

func publishIndex(ctx context.Context, w *kafka.Writer, it IndexTick, cfg Config) error {
		data, err := json.Marshal(it)
		if err != nil {
			return err
		}
		err = w.WriteMessages(ctx, kafka.Message{
			Key: []byte(it.IndexID),
			Topic: cfg.TopicIndex,
			Value: data,
			Time: time.UnixMilli(it.SourceTsMs),
		})
		if err != nil {
			return err
		}
		fmt.Printf("%s value=%.2f avg60s=%.2f (n=%d)\n", it.IndexID, it.Value, it.Avg60s, it.Avg60sWindow)
		return nil
}

