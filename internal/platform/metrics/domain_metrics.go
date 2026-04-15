package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	linkCreatesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDomain,
			Name:      "link_creates_total",
			Help:      "Total number of link create attempts by result.",
		},
		serviceResultLabels,
	)
	redirectResolvesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDomain,
			Name:      "redirect_resolves_total",
			Help:      "Total number of redirect resolutions by result.",
		},
		serviceResultLabels,
	)
	redirectCacheLookupsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystemDomain,
			Name:      "redirect_cache_lookups_total",
			Help:      "Total number of redirect cache lookup outcomes.",
		},
		serviceResultLabels,
	)
)

func registerDomainMetrics() {
	mustRegister(
		linkCreatesTotal,
		redirectResolvesTotal,
		redirectCacheLookupsTotal,
	)
}

func ObserveLinkCreate(service, result string) {
	linkCreatesTotal.WithLabelValues(service, result).Inc()
}

func ObserveRedirectResolve(service, result string) {
	redirectResolvesTotal.WithLabelValues(service, result).Inc()
}

func ObserveRedirectCache(service, result string) {
	redirectCacheLookupsTotal.WithLabelValues(service, result).Inc()
}
