// Package link exposes the link CRUD HTTP API.
//
// Each feature owns its own subpackage under transport/http/stdlib/. The
// package contains a Handler struct (its dependencies), a constructor, a
// Register method that wires its routes onto an http.ServeMux, and the
// handler methods themselves, all in one file unless it gets unwieldy.
package link

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/platform/httppolicy"
	"sandbox_url_shortener/internal/platform/requestmeta"
	"sandbox_url_shortener/services/shortener/internal/domain"
	"sandbox_url_shortener/services/shortener/internal/transport/http/stdlib/respond"
)

// createRateLimitNamespace is the key prefix for per-IP create rate limits.
const createRateLimitNamespace = "ratelimit:create"

// Handler holds the dependencies needed to serve the link CRUD API.
type Handler struct {
	svc           Service
	log           *slog.Logger
	baseURL       string
	idempotency   IdempotencyStore  // optional response cache for Idempotency-Key
	rateLimiter httppolicy.Limiter // optional per-IP limiter for fresh creates
}

// Service is the exact slice of business logic the link CRUD transport needs.
// It lives with the consumer so the transport does not depend on a wider
// service-shaped interface than necessary.
type Service interface {
	Create(ctx context.Context, input domain.CreateLinkInput) (*domain.Link, error)
	ListRecent(ctx context.Context) ([]*domain.Link, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error)
	Disable(ctx context.Context, id uuid.UUID) (*domain.Link, error)
	Enable(ctx context.Context, id uuid.UUID) (*domain.Link, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// NewHandler creates a new link HTTP handler.
func NewHandler(
	svc Service,
	log *slog.Logger,
	baseURL string,
	idempotency IdempotencyStore,
	rateLimiter httppolicy.Limiter,
) *Handler {
	return &Handler{
		svc:           svc,
		log:           log,
		baseURL:       baseURL,
		idempotency:   idempotency,
		rateLimiter: rateLimiter,
	}
}

// Register wires the link routes onto the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/links", h.Create)
	mux.HandleFunc("GET /api/v1/links", h.ListRecent)
	mux.HandleFunc("GET /api/v1/links/{id}", h.GetByID)
	mux.HandleFunc("POST /api/v1/links/{id}/disable", h.Disable)
	mux.HandleFunc("POST /api/v1/links/{id}/enable", h.Enable)
	mux.HandleFunc("DELETE /api/v1/links/{id}", h.Delete)
}

// Create handles POST /api/v1/links. Idempotency-Key replay/conflict runs
// first so cached replays bypass the rate limiter. Fresh creates then check
// the per-IP limiter inside createInner before spending work.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	fresh := http.HandlerFunc(h.createInner)
	if key := r.Header.Get("Idempotency-Key"); key != "" && h.idempotency != nil {
		h.createWithIdempotency(w, r, key, fresh)
		return
	}
	fresh.ServeHTTP(w, r)
}

// createWithIdempotency wraps a fresh-create handler with the Stripe-style key flow.
func (h *Handler) createWithIdempotency(w http.ResponseWriter, r *http.Request, key string, fresh http.Handler) {
	if len(key) > maxIdempotencyKeyLen {
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_idempotency_key",
			"Idempotency-Key exceeds maximum length")
		return
	}

	body, err := readAllAndRestore(r)
	if err != nil {
		h.log.WarnContext(r.Context(), "failed to read body for idempotency", "error", err)
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_body", "failed to read request body")
		return
	}
	bodyHash := hashBody(body)
	storeKey := IdempotencyStoreKey(key)

	rec, err := h.idempotency.Get(r.Context(), storeKey)
	switch {
	case err != nil:
		h.log.WarnContext(r.Context(), "idempotency get failed; processing as fresh",
			"error", err, "key", key)
	case rec != nil && rec.BodyHash == bodyHash:
		h.log.InfoContext(r.Context(), "idempotency replay", "key", key)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rec.StatusCode)
		if _, err := w.Write(rec.ResponseBody); err != nil {
			return
		}
		return
	case rec != nil && rec.BodyHash != bodyHash:
		h.log.InfoContext(r.Context(), "idempotency conflict", "key", key)
		respond.Error(r.Context(), w, http.StatusConflict, "idempotency_conflict",
			"Idempotency-Key reused with a different request body")
		return
	}

	buf := newBufferingWriter()
	fresh.ServeHTTP(buf, r)

	// Persist only successful creates (2xx). Errors are not idempotent: a
	// transient 5xx must not pin a permanent failure to the key.
	if buf.status >= 200 && buf.status < 300 {
		store := &IdempotencyRecord{
			BodyHash:     bodyHash,
			StatusCode:   buf.status,
			ResponseBody: append(json.RawMessage(nil), buf.body.Bytes()...),
		}
		if err := h.idempotency.Set(r.Context(), storeKey, store, idempotencyTTL); err != nil {
			h.log.WarnContext(r.Context(), "idempotency set failed", "error", err, "key", key)
		}
	}
	buf.flushTo(w)
}

// createInner is the fresh-create flow. It enforces the per-IP rate limit
// (if configured), then decodes, validates, and calls the service.
// Limiter errors fail open: a warning is logged and the request proceeds.
func (h *Handler) createInner(w http.ResponseWriter, r *http.Request) {
	if h.rateLimiter != nil {
		key := httppolicy.ClientIPKey(createRateLimitNamespace, r)
		decision, err := h.rateLimiter.Allow(r.Context(), key)
		switch {
		case err != nil:
			h.log.WarnContext(r.Context(), "rate limit backend error; failing open",
				"error", err, "key", key)
		case !decision.Allowed:
			h.log.InfoContext(r.Context(), "rate limit exceeded",
				"key", key, "retry_after", decision.RetryAfter)
			httppolicy.WriteRateLimited(w, requestmeta.RequestID(r.Context()), decision.RetryAfter)
			return
		}
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.DebugContext(r.Context(), "create link request rejected: invalid json")
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_json", "malformed JSON body")
		return
	}

	if strings.TrimSpace(req.OriginalURL) == "" {
		h.log.DebugContext(r.Context(), "create link request rejected: original_url missing")
		respond.Error(r.Context(), w, http.StatusBadRequest, "validation_error", "original_url is required")
		return
	}

	input := domain.CreateLinkInput{
		OriginalURL: req.OriginalURL,
		Title:       req.Title,
		ExpiresAt:   req.ExpiresAt,
	}
	if req.MaxClicks != nil {
		input.MaxClicks = *req.MaxClicks
	}

	h.log.DebugContext(r.Context(), "create link request accepted",
		slog.Bool("has_title", req.Title != nil),
		slog.Bool("has_expires_at", req.ExpiresAt != nil),
		slog.Int64("max_clicks", input.MaxClicks),
	)

	link, err := h.svc.Create(r.Context(), input)
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to create link", "error", err)
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to create link")
		return
	}

	h.log.InfoContext(r.Context(), "link created",
		slog.String("link_id", link.ID.String()),
		slog.String("short_code", link.ShortCode),
		slog.Int64("max_clicks", link.MaxClicks),
		slog.Bool("has_expires_at", link.ExpiresAt != nil),
	)
	respond.JSON(w, http.StatusCreated, toResponse(link, h.baseURL))
}

// GetByID handles GET /api/v1/links/{id}.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.log.DebugContext(r.Context(), "get link request rejected: invalid id", slog.String("id", idStr))
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	h.log.DebugContext(r.Context(), "fetching link by id", slog.String("link_id", id.String()))
	link, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), "link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to get link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}

	h.log.DebugContext(r.Context(), "link fetched", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	respond.JSON(w, http.StatusOK, toResponse(link, h.baseURL))
}

// ListRecent handles GET /api/v1/links.
func (h *Handler) ListRecent(w http.ResponseWriter, r *http.Request) {
	h.log.DebugContext(r.Context(), "listing recent links")
	links, err := h.svc.ListRecent(r.Context())
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to list recent links", "error", err)
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to list links")
		return
	}

	items := make([]Response, 0, len(links))
	for _, link := range links {
		items = append(items, toResponse(link, h.baseURL))
	}
	respond.JSON(w, http.StatusOK, ListResponse{Items: items})
}

// Disable handles POST /api/v1/links/{id}/disable.
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	h.changeState(w, r, "disable", h.svc.Disable)
}

// Enable handles POST /api/v1/links/{id}/enable.
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) {
	h.changeState(w, r, "enable", h.svc.Enable)
}

// changeState applies a state transition like enable/disable and logs the outcome.
func (h *Handler) changeState(
	w http.ResponseWriter,
	r *http.Request,
	action string,
	changeFn func(context.Context, uuid.UUID) (*domain.Link, error),
) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	h.log.DebugContext(r.Context(), action+" link", slog.String("link_id", id.String()))
	link, err := changeFn(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), action+" skipped: link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to "+action+" link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to "+action+" link")
		return
	}

	h.log.InfoContext(r.Context(), "link "+action+"d",
		slog.String("link_id", link.ID.String()),
		slog.String("short_code", link.ShortCode),
	)
	respond.JSON(w, http.StatusOK, toResponse(link, h.baseURL))
}

// Delete handles DELETE /api/v1/links/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	h.log.DebugContext(r.Context(), "deleting link", slog.String("link_id", id.String()))
	err = h.svc.SoftDelete(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), "delete skipped: link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to delete link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to delete link")
		return
	}

	h.log.InfoContext(r.Context(), "link deleted", slog.String("link_id", id.String()))
	w.WriteHeader(http.StatusNoContent)
}

// parseID extracts and validates the {id} path parameter.
func (h *Handler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.log.DebugContext(r.Context(), "request rejected: invalid uuid", slog.String("id", idStr))
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return uuid.Nil, err
	}
	return id, nil
}
