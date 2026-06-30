package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpDurationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	//     Name: "data_collector_http_duration_seconds_gauge",
	//     Help: "Response time for HTTP API requests (Gauge for HW3)",
	// }, []string{"service", "node", "method", "endpoint"})

	httpDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "data_collector_http_duration_seconds",
		Help:    "Histogram of response latency (seconds) for HTTP API requests",
		Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10}, // Timeout API esterne potrebbero essere più lunghi
	}, []string{"service", "node", "method", "endpoint"})

	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "data_collector_http_requests_total",
		Help: "Total HTTP requests received",
	}, []string{"service", "node", "method", "endpoint", "status"})

	inFlightHTTPRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_in_flight_http_requests",
		Help: "Current number of HTTP requests being served",
	}, []string{"service", "node"})

	jobDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_job_duration_seconds",
		Help: "Time taken to complete the data collection cycle",
	}, []string{"service", "node", "status"})

	lastJobTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_last_job_timestamp_seconds",
		Help: "Unix timestamp of the last completed job",
	}, []string{"service", "node"})

	jobErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "data_collector_job_errors_total",
		Help: "Total number of failed data collection cycles",
	}, []string{"service", "node"})

	cbState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "data_collector_circuit_breaker_state",
		Help: "Current state of the Circuit Breaker (0=Closed, 1=Half-Open, 2=Open)",
	}, []string{"service", "node", "target"}) // target può essere "opensky" o "user_manager"

	// AGGIUNTA CIRCUIT BREAKER: Richieste rifiutate
	cbRejectedRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "data_collector_circuit_breaker_rejected_total",
		Help: "Total number of requests rejected due to Open Circuit Breaker",
	}, []string{"service", "node", "target"})
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
	// httpDurationGauge.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Set(duration)
	httpDurationHistogram.WithLabelValues(m.serviceName, m.nodeName, method, endpoint).Observe(duration)
}

func (m *Monitor) IncHTTP(method, endpoint, status string) {
	httpRequests.WithLabelValues(m.serviceName, m.nodeName, method, endpoint, status).Inc()
}

func (m *Monitor) IncInFlightHTTP() {
	inFlightHTTPRequests.WithLabelValues(m.serviceName, m.nodeName).Inc()
}

func (m *Monitor) DecInFlightHTTP() {
	inFlightHTTPRequests.WithLabelValues(m.serviceName, m.nodeName).Dec()
}

func (m *Monitor) ObserveJobDuration(status string, duration float64) {
	jobDuration.WithLabelValues(m.serviceName, m.nodeName, status).Set(duration)
}

func (m *Monitor) SetLastJobTimestamp(ts float64) {
	lastJobTimestamp.WithLabelValues(m.serviceName, m.nodeName).Set(ts)
}

func (m *Monitor) IncJobError() {
	jobErrors.WithLabelValues(m.serviceName, m.nodeName).Inc()
}

func (m *Monitor) SetCBState(target string, stateCode float64) {
	cbState.WithLabelValues(m.serviceName, m.nodeName, target).Set(stateCode)
}

func (m *Monitor) IncCBRejected(target string) {
	cbRejectedRequests.WithLabelValues(m.serviceName, m.nodeName, target).Inc()
}
