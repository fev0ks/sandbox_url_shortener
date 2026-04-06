// Package metrics provides a placeholder metrics endpoint.
// Real Prometheus instrumentation is added in E08.
package metrics

import "net/http"

// Handler returns a placeholder /metrics endpoint that responds with an
// empty Prometheus exposition body.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		// Empty exposition body; real metrics added in E08.
	}
}
