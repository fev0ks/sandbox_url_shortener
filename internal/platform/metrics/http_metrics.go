package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests handled by the service.",
		},
		httpRequestLabels,
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemHTTP,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		httpDurationLabels,
	)
)

func registerHTTPMetrics() {
	mustRegister(
		httpRequestsTotal,
		httpRequestDuration,
	)
}
