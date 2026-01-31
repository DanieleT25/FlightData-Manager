package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	processingTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "user_manager_processing_time_seconds",
		Help: "Time taken to perform the operation in seconds",
	}, []string{"service", "node", "method", "endpoint"})

	requestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "user_manager_requests_total",
		Help: "Total number of requests received",
	}, []string{"service", "node", "method", "endpoint", "status"})
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
	processingTime.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Set(duration)
}

func (m *Monitor) IncRequest(method, endpoint, status string) {
	requestCounter.WithLabelValues(m.serviceName, m.nodeName, method, endpoint, status).Inc()
}
