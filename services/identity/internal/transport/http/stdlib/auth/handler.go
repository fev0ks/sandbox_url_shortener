// Package auth exposes identity authentication HTTP routes.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"sandbox_url_shortener/internal/platform/httppolicy"
	"sandbox_url_shortener/internal/platform/requestmeta"
	"sandbox_url_shortener/services/identity/internal/domain"
	"sandbox_url_shortener/services/identity/internal/service"
	"sandbox_url_shortener/services/identity/internal/transport/http/stdlib/respond"
)

const (
	oauthStateCookiePrefix = "oauth_state_"
	oauthPKCECookiePrefix  = "oauth_pkce_"
)

const (
	routeCSRF           = "GET /api/v1/auth/csrf"
	routeSignup         = "POST /api/v1/auth/signup"
	routeLogin          = "POST /api/v1/auth/login"
	routeSession        = "GET /api/v1/auth/session"
	routeValidate       = "POST /api/v1/auth/validate"
	routeLogout         = "POST /api/v1/auth/logout"
	routeGoogleStart    = "GET /api/v1/auth/oauth/google/start"
	routeGoogleCallback = "GET /api/v1/auth/oauth/google/callback"
	routeAppleStart     = "GET /api/v1/auth/oauth/apple/start"
	routeAppleCallback  = "POST /api/v1/auth/oauth/apple/callback"
)

const (
	queryCode      = "code"
	queryError     = "error"
	queryRequestID = "request_id"
	queryState     = "state"
	queryStatus    = "status"
	queryUser      = "user"
)

const (
	responseStatusOK      = "ok"
	responseStatusSuccess = "success"
)

const (
	authErrorAccountDisabled      = "account_disabled"
	authErrorAccountExists        = "account_exists"
	authErrorInternal             = "internal_error"
	authErrorInvalidCSRF          = "invalid_csrf"
	authErrorInvalidJSON          = "invalid_json"
	authErrorInvalidOAuthForm     = "invalid_oauth_form"
	authErrorInvalidOAuthState    = "invalid_oauth_state"
	authErrorOAuthLinkingBlocked  = "oauth_linking_blocked"
	authErrorOAuthProviderError   = "oauth_provider_error"
	authErrorOAuthProviderInvalid = "oauth_provider_invalid"
	authErrorOAuthStateExpired    = "oauth_state_expired"
	authErrorOAuthUnconfigured    = "oauth_unconfigured"
	authErrorUnauthorized         = "unauthorized"
	authErrorValidation           = "validation_error"
)

const (
	operationAuthCSRF = "auth_csrf"
	operationAuthHTTP = "auth_http"
)

// AuthService is the business contract consumed by the auth transport.
type AuthService interface {
	Signup(ctx context.Context, input domain.SignupInput, currentToken string) (*service.IssuedSession, error)
	Login(ctx context.Context, input domain.LoginInput) (*service.IssuedSession, error)
	CurrentSession(ctx context.Context, rawToken string, now time.Time) (*domain.SessionAccount, error)
	Logout(ctx context.Context, rawToken string, now time.Time, clientIP, userAgent string) error
}

// OAuthService is the OAuth contract consumed by the auth transport.
type OAuthService interface {
	Start(ctx context.Context, providerName, redirectURL string, now time.Time) (*service.OAuthStart, error)
	Complete(ctx context.Context, providerName, state, code, codeVerifier, userJSON, currentToken, clientIP, userAgent string, now time.Time) (*service.IssuedSession, string, error)
}

// Handler holds dependencies needed to serve auth routes.
type Handler struct {
	auth               AuthService
	oauth              OAuthService
	log                *slog.Logger
	webCallbackURL     string
	cookieSecure       bool
	cookieDomain       string
	absoluteSessionTTL time.Duration
}

// NewHandler creates a new auth HTTP handler.
func NewHandler(
	authSvc AuthService,
	oauthSvc OAuthService,
	log *slog.Logger,
	webCallbackURL string,
	cookieSecure bool,
	cookieDomain string,
	absoluteSessionTTL time.Duration,
) *Handler {
	return &Handler{
		auth:               authSvc,
		oauth:              oauthSvc,
		log:                log,
		webCallbackURL:     webCallbackURL,
		cookieSecure:       cookieSecure,
		cookieDomain:       cookieDomain,
		absoluteSessionTTL: absoluteSessionTTL,
	}
}

// Register wires auth routes onto the mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc(routeCSRF, h.CSRF)
	mux.HandleFunc(routeSignup, h.Signup)
	mux.HandleFunc(routeLogin, h.Login)
	mux.HandleFunc(routeSession, h.CurrentSession)
	mux.HandleFunc(routeValidate, h.CurrentSession)
	mux.HandleFunc(routeLogout, h.Logout)
	mux.HandleFunc(routeGoogleStart, h.GoogleStart)
	mux.HandleFunc(routeGoogleCallback, h.GoogleCallback)
	mux.HandleFunc(routeAppleStart, h.AppleStart)
	mux.HandleFunc(routeAppleCallback, h.AppleCallback)
}

// CSRF issues a non-HttpOnly double-submit token companion cookie.
func (h *Handler) CSRF(w http.ResponseWriter, r *http.Request) {
	token, err := service.NewCSRFToken()
	if err != nil {
		h.log.ErrorContext(r.Context(), "csrf token failed", "operation", operationAuthCSRF, "error", err)
		respond.Error(r.Context(), w, http.StatusInternalServerError, authErrorInternal, "failed to issue csrf token")
		return
	}
	http.SetCookie(w, h.csrfCookie(token, service.CSRFBootstrapTTL))
	respond.JSON(w, http.StatusOK, csrfResponse{
		CSRFToken: token,
		Header:    service.CSRFHeaderName,
		Cookie:    service.CSRFCookieName,
	})
}

// Signup handles POST /api/v1/auth/signup.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if !h.validCSRF(w, r) {
		return
	}
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(r.Context(), w, http.StatusBadRequest, authErrorInvalidJSON, "malformed JSON body")
		return
	}
	issued, err := h.auth.Signup(r.Context(), domain.SignupInput{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Password:    req.Password,
		Now:         time.Now().UTC(),
		ClientIP:    httppolicy.ClientIP(r),
		UserAgent:   r.UserAgent(),
	}, h.sessionCookieValue(r))
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setIssuedCookies(w, issued)
	respond.JSON(w, http.StatusCreated, h.sessionResponse(issued))
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.validCSRF(w, r) {
		return
	}
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(r.Context(), w, http.StatusBadRequest, authErrorInvalidJSON, "malformed JSON body")
		return
	}
	issued, err := h.auth.Login(r.Context(), domain.LoginInput{
		Email:        req.Email,
		Password:     req.Password,
		Now:          time.Now().UTC(),
		CurrentToken: h.sessionCookieValue(r),
		ClientIP:     httppolicy.ClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setIssuedCookies(w, issued)
	respond.JSON(w, http.StatusOK, h.sessionResponse(issued))
}

// CurrentSession handles current-session and gateway validation checks.
func (h *Handler) CurrentSession(w http.ResponseWriter, r *http.Request) {
	sessionAccount, err := h.auth.CurrentSession(r.Context(), h.sessionCookieValue(r), time.Now().UTC())
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, sessionResponse{
		Account: toAccountResponse(sessionAccount.Account, sessionAccount.Providers),
		Session: sessionInfo{ExpiresAt: sessionAccount.Session.ExpiresAt},
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if !h.validCSRF(w, r) {
		return
	}
	if err := h.auth.Logout(r.Context(), h.sessionCookieValue(r), time.Now().UTC(), httppolicy.ClientIP(r), r.UserAgent()); err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	http.SetCookie(w, h.expiredCookie(service.SessionCookieName, true))
	http.SetCookie(w, h.expiredCookie(service.CSRFCookieName, false))
	respond.JSON(w, http.StatusOK, map[string]string{queryStatus: responseStatusOK})
}

func (h *Handler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	h.oauthStart(w, r, domain.ProviderGoogle)
}

func (h *Handler) AppleStart(w http.ResponseWriter, r *http.Request) {
	h.oauthStart(w, r, domain.ProviderApple)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get(queryState)
	code := r.URL.Query().Get(queryCode)
	if providerErr := r.URL.Query().Get(queryError); providerErr != "" {
		h.redirectResult(w, r, h.webCallbackURL, authErrorOAuthProviderError)
		return
	}
	h.oauthCallback(w, r, domain.ProviderGoogle, state, code, "")
}

func (h *Handler) AppleCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectResult(w, r, h.webCallbackURL, authErrorInvalidOAuthForm)
		return
	}
	if providerErr := r.Form.Get(queryError); providerErr != "" {
		h.redirectResult(w, r, h.webCallbackURL, authErrorOAuthProviderError)
		return
	}
	h.oauthCallback(w, r, domain.ProviderApple, r.Form.Get(queryState), r.Form.Get(queryCode), r.Form.Get(queryUser))
}

func (h *Handler) oauthStart(w http.ResponseWriter, r *http.Request, provider string) {
	start, err := h.oauth.Start(r.Context(), provider, h.webCallbackURL, time.Now().UTC())
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	if provider != domain.ProviderApple {
		http.SetCookie(w, h.oauthStateCookie(provider, start.State))
	}
	http.SetCookie(w, h.oauthPKCECookie(provider, start.PKCEVerifier))
	http.Redirect(w, r, start.RedirectURL, http.StatusFound)
}

func (h *Handler) oauthCallback(w http.ResponseWriter, r *http.Request, provider, state, code, userJSON string) {
	if state == "" {
		h.redirectResult(w, r, h.webCallbackURL, authErrorInvalidOAuthState)
		return
	}
	if cookie, err := r.Cookie(oauthStateCookiePrefix + provider); err == nil && cookie.Value != "" && cookie.Value != state {
		h.redirectResult(w, r, h.webCallbackURL, authErrorInvalidOAuthState)
		return
	}
	pkceCookie, err := r.Cookie(oauthPKCECookiePrefix + provider)
	if err != nil || pkceCookie.Value == "" {
		h.redirectResult(w, r, h.webCallbackURL, authErrorOAuthProviderInvalid)
		return
	}
	issued, redirectURL, err := h.oauth.Complete(
		r.Context(),
		provider,
		state,
		code,
		pkceCookie.Value,
		userJSON,
		h.sessionCookieValue(r),
		httppolicy.ClientIP(r),
		r.UserAgent(),
		time.Now().UTC(),
	)
	if err != nil {
		target := redirectURL
		if target == "" {
			target = h.webCallbackURL
		}
		h.redirectResult(w, r, target, errorCode(err))
		return
	}
	h.setIssuedCookies(w, issued)
	http.SetCookie(w, h.expiredCookie(oauthStateCookiePrefix+provider, true))
	http.SetCookie(w, h.expiredCookie(oauthPKCECookiePrefix+provider, true))
	h.redirectResult(w, r, redirectURL, "")
}

func (h *Handler) validCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(service.CSRFCookieName)
	if err != nil || service.ValidateCSRF(cookie.Value, r.Header.Get(service.CSRFHeaderName)) != nil {
		respond.Error(r.Context(), w, http.StatusForbidden, authErrorInvalidCSRF, "invalid csrf token")
		return false
	}
	return true
}

func (h *Handler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, authErrorInternal, "identity request failed"
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status, code, message = http.StatusBadRequest, authErrorValidation, err.Error()
	case errors.Is(err, domain.ErrAccountExists):
		status, code, message = http.StatusConflict, authErrorAccountExists, "account already exists"
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrSessionNotFound), errors.Is(err, domain.ErrSessionExpired), errors.Is(err, domain.ErrSessionRevoked):
		status, code, message = http.StatusUnauthorized, authErrorUnauthorized, "authentication required"
	case errors.Is(err, domain.ErrInvalidCSRF):
		status, code, message = http.StatusForbidden, authErrorInvalidCSRF, "invalid csrf token"
	case errors.Is(err, domain.ErrAccountDisabled):
		status, code, message = http.StatusForbidden, authErrorAccountDisabled, "account is disabled"
	case errors.Is(err, domain.ErrOAuthUnconfigured):
		status, code, message = http.StatusServiceUnavailable, authErrorOAuthUnconfigured, "oauth provider is not configured"
	case errors.Is(err, domain.ErrOAuthLinkingBlocked):
		status, code, message = http.StatusConflict, authErrorOAuthLinkingBlocked, "provider account could not be linked safely"
	case errors.Is(err, domain.ErrOAuthProviderInvalid), errors.Is(err, domain.ErrOAuthStateNotFound), errors.Is(err, domain.ErrOAuthStateExpired):
		status, code, message = http.StatusBadRequest, errorCode(err), "oauth authentication failed"
	}
	if status >= 500 {
		h.log.ErrorContext(r.Context(), "identity handler error", "operation", operationAuthHTTP, "error", err)
	}
	respond.Error(r.Context(), w, status, code, message)
}

func (h *Handler) setIssuedCookies(w http.ResponseWriter, issued *service.IssuedSession) {
	http.SetCookie(w, h.sessionCookie(issued.Token, issued.Session.ExpiresAt))
	csrf, err := service.NewCSRFToken()
	if err == nil {
		http.SetCookie(w, h.csrfCookie(csrf, time.Until(issued.Session.ExpiresAt)))
	}
}

func (h *Handler) sessionCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(service.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) sessionResponse(issued *service.IssuedSession) sessionResponse {
	return sessionResponse{
		Account: toAccountResponse(issued.Account, issued.Providers),
		Session: sessionInfo{ExpiresAt: issued.Session.ExpiresAt},
	}
}

func toAccountResponse(account *domain.Account, providers []string) accountResponse {
	return accountResponse{
		ID:            account.ID.String(),
		Email:         account.Email,
		DisplayName:   account.DisplayName,
		EmailVerified: account.EmailVerifiedAt != nil,
		Providers:     providers,
	}
}

func (h *Handler) sessionCookie(value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     service.SessionCookieName,
		Value:    value,
		Path:     service.SessionCookiePath,
		Domain:   h.cookieDomain,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) csrfCookie(value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     service.CSRFCookieName,
		Value:    value,
		Path:     service.SessionCookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) oauthStateCookie(provider, value string) *http.Cookie {
	return &http.Cookie{
		Name:     oauthStateCookiePrefix + provider,
		Value:    value,
		Path:     service.SessionCookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   int(service.OAuthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) oauthPKCECookie(provider, value string) *http.Cookie {
	sameSite := http.SameSiteLaxMode
	if provider == domain.ProviderApple {
		sameSite = http.SameSiteNoneMode
	}
	return &http.Cookie{
		Name:     oauthPKCECookiePrefix + provider,
		Value:    value,
		Path:     service.SessionCookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   int(service.OAuthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: sameSite,
	}
}

func (h *Handler) expiredCookie(name string, httpOnly bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     service.SessionCookiePath,
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: httpOnly,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) redirectResult(w http.ResponseWriter, r *http.Request, target, code string) {
	parsed, err := url.Parse(target)
	if err != nil {
		fallback, parseErr := url.Parse(h.webCallbackURL)
		if parseErr != nil {
			respond.Error(r.Context(), w, http.StatusInternalServerError, authErrorInternal, "invalid auth callback url")
			return
		}
		parsed = fallback
	}
	q := parsed.Query()
	if code == "" {
		q.Set(queryStatus, responseStatusSuccess)
	} else {
		q.Set(queryError, code)
		if requestID := requestmeta.RequestID(r.Context()); requestID != "" {
			q.Set(queryRequestID, requestID)
		}
	}
	parsed.RawQuery = q.Encode()
	http.Redirect(w, r, parsed.String(), http.StatusFound)
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrOAuthLinkingBlocked):
		return authErrorOAuthLinkingBlocked
	case errors.Is(err, domain.ErrOAuthStateExpired):
		return authErrorOAuthStateExpired
	case errors.Is(err, domain.ErrOAuthStateNotFound):
		return authErrorInvalidOAuthState
	case errors.Is(err, domain.ErrOAuthUnconfigured):
		return authErrorOAuthUnconfigured
	default:
		return authErrorOAuthProviderInvalid
	}
}
