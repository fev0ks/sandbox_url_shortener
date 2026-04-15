package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	analyticsConsumerEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemAnalytics,
			Name:      "consumer_events_total",
			Help:      "Total number of analytics consumer outcomes by phase and result.",
		},
		servicePhaseResultLabels,
	)
	clickhouseOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemAnalytics,
			Name:      "clickhouse_operations_total",
			Help:      "Total number of ClickHouse operations by name and result.",
		},
		serviceOperationResultLabels,
	)
	clickhouseOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemAnalytics,
			Name:      "clickhouse_operation_duration_seconds",
			Help:      "Duration of ClickHouse operations in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		serviceOperationResultLabels,
	)
)

func registerAnalyticsMetrics() {
	mustRegister(
		analyticsConsumerEventsTotal,
		clickhouseOperationsTotal,
		clickhouseOperationDuration,
	)
}

func ObserveAnalyticsConsumer(service, phase, result string) {
	analyticsConsumerEventsTotal.WithLabelValues(service, phase, result).Inc()
}

func ObserveClickHouseOperation(service, operation, result string, duration time.Duration) {
	clickhouseOperationsTotal.WithLabelValues(service, operation, result).Inc()
	clickhouseOperationDuration.WithLabelValues(service, operation, result).Observe(duration.Seconds())
}
