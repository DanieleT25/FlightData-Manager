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

	err := w.app.RunCollectionCycle(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		log.Printf("Collection cycle failed after %.2f seconds: %v", duration, err)
		w.monitor.ObserveJobDuration("error", duration)
		w.monitor.IncJobError()
	} else {
		log.Printf("Collection cycle finished successfully in %.2f seconds", duration)

		// Tutto liscio: registriamo il successo e aggiorniamo il timestamp
		w.monitor.ObserveJobDuration("success", duration)
		w.monitor.SetLastJobTimestamp(float64(time.Now().Unix()))
	}
}
