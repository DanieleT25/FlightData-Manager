package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func NewMetricsMiddleware(monitor *observability.Monitor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			monitor.IncInFlightHTTP()
			defer monitor.DecInFlightHTTP()

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)
			duration := time.Since(start).Seconds()
			statusStr := strconv.Itoa(rec.statusCode)
			endpoint := r.URL.Path

			monitor.ObserveHTTP(r.Method, endpoint, duration)
			monitor.IncHTTP(r.Method, endpoint, statusStr)
		})
	}
}
