package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sandbox_url_shortener/internal/platform/middleware"
)

const (
	metricsNamespace = "sandbox_url_shortener"

	metricsSubsystemHTTP      = "http"
	metricsSubsystemDomain    = "domain"
	metricsSubsystemEventing  = "eventing"
	metricsSubsystemAnalytics = "analytics"
	metricsSubsystemStorage   = "storage"

	routeUnknown   = "unknown"
	routeUnmatched = "unmatched"

	labelService    = "service"
	labelMethod     = "method"
	labelRoute      = "route"
	labelStatusCode = "status_code"
	labelResult     = "result"
	labelOperation  = "operation"
	labelPhase      = "phase"
)

var (
	httpRequestLabels            = []string{labelService, labelMethod, labelRoute, labelStatusCode}
	httpDurationLabels           = []string{labelService, labelMethod, labelRoute}
	serviceResultLabels          = []string{labelService, labelResult}
	servicePhaseResultLabels     = []string{labelService, labelPhase, labelResult}
	serviceOperationResultLabels = []string{labelService, labelOperation, labelResult}
)

func init() {
	registerHTTPMetrics()
	registerDomainMetrics()
	registerEventingMetrics()
	registerAnalyticsMetrics()
	registerStorageMetrics()
}

// Handler exposes the Prometheus registry for scraping.
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTPMiddleware records per-request count and duration metrics.
func HTTPMiddleware(service string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipHTTPObservability(r) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)

			route := httpRouteLabel(r)
			code := strconv.Itoa(rec.code)
			httpRequestsTotal.WithLabelValues(service, r.Method, route, code).Inc()
			httpRequestDuration.WithLabelValues(service, r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

func mustRegister(collectors ...prometheus.Collector) {
	prometheus.MustRegister(collectors...)
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func httpRouteLabel(r *http.Request) string {
	if r == nil {
		return routeUnknown
	}
	if pattern := r.Pattern; pattern != "" {
		return pattern
	}
	return fmt.Sprintf("%s %s", r.Method, routeUnmatched)
}

func shouldSkipHTTPObservability(r *http.Request) bool {
	if r == nil {
		return false
	}
	return r.URL.Path == "/metrics" || strings.HasPrefix(r.URL.Path, "/health/")
}
