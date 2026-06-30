package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// processingTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
	// 	Name: "user_manager_processing_time_seconds",
	// 	Help: "Time taken to perform the operation in seconds",
	// }, []string{"service", "node", "method", "endpoint"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "user_manager_request_duration_seconds",
		Help:    "Histogram of response latency (seconds) of requests.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"service", "node", "method", "endpoint"})

	requestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "user_manager_requests_total",
		Help: "Total number of requests received",
	}, []string{"service", "node", "method", "endpoint", "status"})

	inFlightRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "user_manager_in_flight_requests",
		Help: "Current number of requests being served.",
	}, []string{"service", "node"})
)

type Monitor struct {
	serviceName string
	nodeName    string
}

func NewMonitor(serviceName, nodeName string) *Monitor {
	return &Monitor{
		serviceName: serviceName,
		nodeName:    nodeName,
	}
}

func (m *Monitor) ObserveDuration(method, endpoint string, duration float64) {
	//processingTime.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Set(duration)
	requestDuration.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Observe(duration)
}

func (m *Monitor) IncRequest(method, endpoint, status string) {
	requestCounter.WithLabelValues(m.serviceName, m.nodeName, method, endpoint, status).Inc()
}

func (m *Monitor) IncInFlight() {
	inFlightRequests.WithLabelValues(m.serviceName, m.nodeName).Inc()
}

func (m *Monitor) DecInFlight() {
	inFlightRequests.WithLabelValues(m.serviceName, m.nodeName).Dec()
}
