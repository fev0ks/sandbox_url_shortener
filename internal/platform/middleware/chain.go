package middleware

import "net/http"

// Middleware is the canonical shape every middleware in this package conforms to.
// A middleware wraps an http.Handler and returns a new one.
type Middleware func(http.Handler) http.Handler

// Compose folds a slice of middlewares around a handler so that the FIRST entry
// in mws runs FIRST at request time. This makes the call site read top-to-bottom
// in execution order — no mental reversal of nested calls.
//
//	handler = Compose(mux,
//	    Recovery,             // 1. runs first
//	    RequestID,            // 2.
//	    Logging(base),        // 3.
//	)
func Compose(handler http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
