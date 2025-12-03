package worker

import (
	"context"
	"log"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/ports"
)

type TickerWorker struct {
	app      ports.CollectorAPI
	interval time.Duration
}

func NewTickerWorker(app ports.CollectorAPI, interval time.Duration) *TickerWorker {
	return &TickerWorker{
		app:      app,
		interval: interval,
	}
}

func (w *TickerWorker) Start(ctx context.Context) {
	log.Printf("Worker Adapter started (Interval: %v)", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.app.RunCollectionCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.app.RunCollectionCycle(ctx)
		}
	}
}
