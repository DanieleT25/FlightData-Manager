package worker

import (
	"context"
	"log"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/ports"
)

type TickerWorker struct {
	app      ports.CollectorAPI
	interval time.Duration
	monitor  *observability.Monitor
}

func NewTickerWorker(app ports.CollectorAPI, interval time.Duration, monitor *observability.Monitor) *TickerWorker {
	return &TickerWorker{
		app:      app,
		interval: interval,
		monitor:  monitor,
	}
}

func (w *TickerWorker) Start(ctx context.Context) {
	log.Printf("Worker Adapter started (Interval: %v)", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.runWithMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runWithMetrics(ctx)
		}
	}
}

func (w *TickerWorker) runWithMetrics(ctx context.Context) {
	start := time.Now()
	log.Println("Starting scheduled collection cycle...")
	w.app.RunCollectionCycle(ctx)
	duration := time.Since(start).Seconds()

	w.monitor.ObserveJobDuration("success", duration)
	w.monitor.SetLastJobTimestamp(float64(time.Now().Unix()))

	log.Printf("Collection cycle finished in %.2f seconds", duration)
}
