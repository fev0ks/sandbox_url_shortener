package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	linkVisitPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemEventing,
			Name:      "link_visit_publish_total",
			Help:      "Total number of link.visited publish outcomes.",
		},
		serviceResultLabels,
	)
)

func registerEventingMetrics() {
	mustRegister(linkVisitPublishTotal)
}

func ObserveLinkVisitPublish(service, result string) {
	linkVisitPublishTotal.WithLabelValues(service, result).Inc()
}
