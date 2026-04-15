package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	postgresOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "postgres_operations_total",
			Help:      "Total number of PostgreSQL operations by name and result.",
		},
		serviceOperationResultLabels,
	)
	postgresOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "postgres_operation_duration_seconds",
			Help:      "Duration of PostgreSQL operations in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		serviceOperationResultLabels,
	)
	redisOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "redis_operations_total",
			Help:      "Total number of Redis operations by name and result.",
		},
		serviceOperationResultLabels,
	)
	redisOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemStorage,
			Name:      "redis_operation_duration_seconds",
			Help:      "Duration of Redis operations in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		serviceOperationResultLabels,
	)
)

func registerStorageMetrics() {
	mustRegister(
		postgresOperationsTotal,
		postgresOperationDuration,
		redisOperationsTotal,
		redisOperationDuration,
	)
}

func ObservePostgresOperation(service, operation, result string, duration time.Duration) {
	postgresOperationsTotal.WithLabelValues(service, operation, result).Inc()
	postgresOperationDuration.WithLabelValues(service, operation, result).Observe(duration.Seconds())
}

func ObserveRedisOperation(service, operation, result string, duration time.Duration) {
	redisOperationsTotal.WithLabelValues(service, operation, result).Inc()
	redisOperationDuration.WithLabelValues(service, operation, result).Observe(duration.Seconds())
}
