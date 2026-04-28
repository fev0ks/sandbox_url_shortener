package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/identity/internal/domain"
)

// OAuthProvider exchanges authorization codes and normalizes provider claims.
type OAuthProvider interface {
	Name() string
	AuthURL(state, codeChallenge, nonce string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier, nonceHash string, userJSON string) (domain.OAuthClaims, error)
}

// OAuthService implements OAuth state, PKCE, nonce, and provider callback orchestration.
type OAuthService struct {
	repo      domain.Repository
	auth      *AuthService
	providers map[string]OAuthProvider
}

// NewOAuthService creates an OAuth service.
func NewOAuthService(repo domain.Repository, auth *AuthService, providers ...OAuthProvider) *OAuthService {
	byName := make(map[string]OAuthProvider, len(providers))
	for _, provider := range providers {
		if provider != nil {
			byName[provider.Name()] = provider
		}
	}
	return &OAuthService{repo: repo, auth: auth, providers: byName}
}

// OAuthStart carries the redirect URL and raw state for cookie correlation.
type OAuthStart struct {
	RedirectURL  string
	State        string
	PKCEVerifier string
}

// Start creates single-use OAuth state and returns the provider authorization URL.
func (s *OAuthService) Start(ctx context.Context, providerName, redirectURL string, now time.Time) (*OAuthStart, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, domain.ErrOAuthUnconfigured
	}
	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	verifier, err := randomToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, err
	}
	now = normalizedNow(now)
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("oauth state uuid: %w", err)
	}
	oauthState := &domain.OAuthState{
		ID:               id,
		Provider:         providerName,
		StateHash:        tokenHash(state),
		PKCEVerifierHash: tokenHash(verifier),
		NonceHash:        tokenHash(nonce),
		RedirectURL:      redirectURL,
		ExpiresAt:        now.Add(OAuthStateTTL),
		CreatedAt:        now,
	}
	if err := s.repo.CreateOAuthState(ctx, oauthState); err != nil {
		return nil, err
	}
	authURL, err := provider.AuthURL(state, pkceChallenge(verifier), nonce)
	if err != nil {
		return nil, err
	}
	return &OAuthStart{RedirectURL: authURL, State: state, PKCEVerifier: verifier}, nil
}

// Complete validates state and provider claims, then issues a shared identity session.
func (s *OAuthService) Complete(ctx context.Context, providerName, state, code, codeVerifier, userJSON, currentToken, clientIP, userAgent string, now time.Time) (*IssuedSession, string, error) {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" || strings.TrimSpace(codeVerifier) == "" {
		return nil, "", domain.ErrOAuthProviderInvalid
	}
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, "", domain.ErrOAuthUnconfigured
	}
	oauthState, err := s.repo.ConsumeOAuthState(ctx, providerName, tokenHash(state), normalizedNow(now))
	if err != nil {
		return nil, "", err
	}
	if !constantStringEqual(tokenHash(codeVerifier), oauthState.PKCEVerifierHash) {
		return nil, oauthState.RedirectURL, domain.ErrOAuthProviderInvalid
	}
	claims, err := provider.Exchange(ctx, code, codeVerifier, oauthState.NonceHash, userJSON)
	if err != nil {
		return nil, oauthState.RedirectURL, err
	}
	issued, err := s.auth.IssueOAuthSession(ctx, domain.OAuthLoginInput{
		Claims:       claims,
		Now:          now,
		CurrentToken: currentToken,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
	})
	if err != nil {
		return nil, oauthState.RedirectURL, err
	}
	return issued, oauthState.RedirectURL, nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func authURL(base string, values url.Values) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

type idTokenClaims struct {
	Issuer        string          `json:"iss"`
	Audience      json.RawMessage `json:"aud"`
	Subject       string          `json:"sub"`
	ExpiresAt     int64           `json:"exp"`
	IssuedAt      int64           `json:"iat"`
	Nonce         string          `json:"nonce"`
	Email         string          `json:"email"`
	EmailVerified json.RawMessage `json:"email_verified"`
}

func decodeIDTokenClaims(idToken string) (idTokenClaims, map[string]any, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return idTokenClaims{}, nil, fmt.Errorf("%w: id_token has invalid shape", domain.ErrOAuthProviderInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return idTokenClaims{}, nil, fmt.Errorf("%w: id_token payload decode", domain.ErrOAuthProviderInvalid)
	}
	var claims idTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return idTokenClaims{}, nil, fmt.Errorf("%w: id_token claims decode", domain.ErrOAuthProviderInvalid)
	}
	var metadata map[string]any
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return idTokenClaims{}, nil, fmt.Errorf("%w: id_token metadata decode", domain.ErrOAuthProviderInvalid)
	}
	return claims, metadata, nil
}

func verifyCommonClaims(claims idTokenClaims, issuer string, audience string, nonceHash string, now time.Time) error {
	if claims.Issuer != issuer {
		return fmt.Errorf("%w: issuer mismatch", domain.ErrOAuthProviderInvalid)
	}
	if !audienceMatches(claims.Audience, audience) {
		return fmt.Errorf("%w: audience mismatch", domain.ErrOAuthProviderInvalid)
	}
	if claims.Subject == "" {
		return fmt.Errorf("%w: subject missing", domain.ErrOAuthProviderInvalid)
	}
	nowUnix := normalizedNow(now).Unix()
	if claims.ExpiresAt+int64(OIDCClockSkewLeeway.Seconds()) <= nowUnix {
		return fmt.Errorf("%w: token expired", domain.ErrOAuthProviderInvalid)
	}
	if claims.IssuedAt > 0 && claims.IssuedAt-int64(OIDCClockSkewLeeway.Seconds()) > nowUnix {
		return fmt.Errorf("%w: token issued in future", domain.ErrOAuthProviderInvalid)
	}
	if claims.Nonce == "" || tokenHash(claims.Nonce) != nonceHash {
		return fmt.Errorf("%w: nonce mismatch", domain.ErrOAuthProviderInvalid)
	}
	return nil
}

func audienceMatches(raw json.RawMessage, want string) bool {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single == want
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err == nil {
		for _, value := range multiple {
			if value == want {
				return true
			}
		}
	}
	return false
}

func boolClaim(raw json.RawMessage) bool {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == JSONTrueString
	}
	return false
}

func metadataBytes(metadata map[string]any) []byte {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return []byte(JSONEmptyObject)
	}
	return payload
}

func oauthErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrOAuthProviderInvalid) ||
		errors.Is(err, domain.ErrOAuthLinkingBlocked) ||
		errors.Is(err, domain.ErrOAuthStateNotFound) ||
		errors.Is(err, domain.ErrOAuthStateExpired) ||
		errors.Is(err, domain.ErrOAuthUnconfigured) {
		return err
	}
	return errors.Join(domain.ErrOAuthProviderInvalid, err)
}
