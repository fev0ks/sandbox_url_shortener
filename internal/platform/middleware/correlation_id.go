package middleware

import "net/http"

const headerCorrelationID = "X-Correlation-ID"

// CorrelationID reads X-Correlation-ID from the request, falls back to request_id,
// stores it in context, and sets it on the response header.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerCorrelationID)
		if id == "" {
			id = GetRequestID(r.Context())
		}

		w.Header().Set(headerCorrelationID, id)

		ctx := withCorrelationID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
