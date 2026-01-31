package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_http_duration_seconds",
		Help: "Response time for HTTP API requests",
	}, []string{"service", "node", "method", "endpoint"})

	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "data_collector_http_requests_total",
		Help: "Total HTTP requests received",
	}, []string{"service", "node", "method", "endpoint", "status"})

	jobDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_job_duration_seconds",
		Help: "Time taken to complete the data collection cycle",
	}, []string{"service", "node", "status"})

	lastJobTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_last_job_timestamp_seconds",
		Help: "Unix timestamp of the last completed job",
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

func (m *Monitor) ObserveHTTP(method, endpoint string, duration float64) {
	httpDuration.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Set(duration)
}

func (m *Monitor) IncHTTP(method, endpoint, status string) {
	httpRequests.WithLabelValues(m.serviceName, m.nodeName, method, endpoint, status).Inc()
}

func (m *Monitor) ObserveJobDuration(status string, duration float64) {
	jobDuration.WithLabelValues(m.serviceName, m.nodeName, status).Set(duration)
}

func (m *Monitor) SetLastJobTimestamp(ts float64) {
	lastJobTimestamp.WithLabelValues(m.serviceName, m.nodeName).Set(ts)
}
