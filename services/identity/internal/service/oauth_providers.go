package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"sandbox_url_shortener/services/identity/internal/config"
	"sandbox_url_shortener/services/identity/internal/domain"
)

const (
	googleAuthEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	//nolint:gosec // Public OAuth endpoint, not a secret.
	googleTokenEndpoint = "https://oauth2.googleapis.com/token"
	googleJWKSURL       = "https://www.googleapis.com/oauth2/v3/certs"
	googleIssuer        = "https://accounts.google.com"

	appleAuthEndpoint = "https://appleid.apple.com/auth/authorize"
	//nolint:gosec // Public OAuth endpoint, not a secret.
	appleTokenEndpoint = "https://appleid.apple.com/auth/token"
	appleJWKSURL       = "https://appleid.apple.com/auth/keys"
	appleIssuer        = "https://appleid.apple.com"
)

type tokenResponse struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

// GoogleProvider implements Google OIDC code exchange.
type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewGoogleProvider creates a Google OAuth provider when config is complete.
func NewGoogleProvider(cfg *config.Config) OAuthProvider {
	if !cfg.GoogleOAuthEnabled() {
		return nil
	}
	return &GoogleProvider{
		clientID:     cfg.GoogleClientID,
		clientSecret: cfg.GoogleClientSecret,
		redirectURL:  cfg.GoogleRedirectURL,
		httpClient:   oauthHTTPClient(),
	}
}

func (p *GoogleProvider) Name() string { return domain.ProviderGoogle }

func (p *GoogleProvider) AuthURL(state, codeChallenge, nonce string) (string, error) {
	return authURL(googleAuthEndpoint, url.Values{
		"client_id":             {p.clientID},
		"redirect_uri":          {p.redirectURL},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	})
}

func (p *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier, nonceHash string, _ string) (domain.OAuthClaims, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"redirect_uri":  {p.redirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier},
	}
	resp, err := postForm(ctx, p.httpClient, googleTokenEndpoint, form)
	if err != nil {
		return domain.OAuthClaims{}, oauthErr(err)
	}
	if err := verifyIDTokenSignature(ctx, p.httpClient, resp.IDToken, googleJWKSURL); err != nil {
		return domain.OAuthClaims{}, oauthErr(err)
	}
	claims, metadata, err := decodeIDTokenClaims(resp.IDToken)
	if err != nil {
		return domain.OAuthClaims{}, err
	}
	if err := verifyCommonClaims(claims, googleIssuer, p.clientID, nonceHash, time.Now()); err != nil {
		return domain.OAuthClaims{}, err
	}
	metadata["provider"] = domain.ProviderGoogle
	return domain.OAuthClaims{
		Provider:        domain.ProviderGoogle,
		ProviderSubject: claims.Subject,
		Email:           claims.Email,
		EmailVerified:   boolClaim(claims.EmailVerified),
		RawMetadata:     metadataBytes(metadata),
	}, nil
}

// AppleProvider implements Sign in with Apple code exchange.
type AppleProvider struct {
	clientID      string
	teamID        string
	keyID         string
	privateKeyPEM string
	redirectURL   string
	httpClient    *http.Client
	mu            sync.Mutex
	cachedSecret  string
	cachedUntil   time.Time
}

// NewAppleProvider creates an Apple OAuth provider when config is complete.
func NewAppleProvider(cfg *config.Config) OAuthProvider {
	if !cfg.AppleOAuthEnabled() {
		return nil
	}
	return &AppleProvider{
		clientID:      cfg.AppleClientID,
		teamID:        cfg.AppleTeamID,
		keyID:         cfg.AppleKeyID,
		privateKeyPEM: cfg.ApplePrivateKeyPEM,
		redirectURL:   cfg.AppleRedirectURL,
		httpClient:    oauthHTTPClient(),
	}
}

func (p *AppleProvider) Name() string { return domain.ProviderApple }

func (p *AppleProvider) AuthURL(state, _ string, nonce string) (string, error) {
	return authURL(appleAuthEndpoint, url.Values{
		"client_id":     {p.clientID},
		"redirect_uri":  {p.redirectURL},
		"response_type": {"code"},
		"response_mode": {"form_post"},
		"scope":         {"name email"},
		"state":         {state},
		"nonce":         {nonce},
	})
}

func (p *AppleProvider) Exchange(ctx context.Context, code, _ string, nonceHash string, userJSON string) (domain.OAuthClaims, error) {
	clientSecret, err := p.clientSecret()
	if err != nil {
		return domain.OAuthClaims{}, oauthErr(err)
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {p.clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {p.redirectURL},
		"grant_type":    {"authorization_code"},
	}
	resp, err := postForm(ctx, p.httpClient, appleTokenEndpoint, form)
	if err != nil {
		return domain.OAuthClaims{}, oauthErr(err)
	}
	if err := verifyIDTokenSignature(ctx, p.httpClient, resp.IDToken, appleJWKSURL); err != nil {
		return domain.OAuthClaims{}, oauthErr(err)
	}
	claims, metadata, err := decodeIDTokenClaims(resp.IDToken)
	if err != nil {
		return domain.OAuthClaims{}, err
	}
	if err := verifyCommonClaims(claims, appleIssuer, p.clientID, nonceHash, time.Now()); err != nil {
		return domain.OAuthClaims{}, err
	}
	displayName := parseAppleUserDisplayName(userJSON)
	email := claims.Email
	metadata["provider"] = domain.ProviderApple
	if displayName != "" {
		metadata["display_name"] = displayName
	}
	return domain.OAuthClaims{
		Provider:        domain.ProviderApple,
		ProviderSubject: claims.Subject,
		Email:           email,
		EmailVerified:   boolClaim(claims.EmailVerified),
		DisplayName:     displayName,
		PrivateRelay:    strings.HasSuffix(strings.ToLower(email), "@privaterelay.appleid.com"),
		RawMetadata:     metadataBytes(metadata),
	}, nil
}

func (p *AppleProvider) clientSecret() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	nowTime := time.Now()
	if p.cachedSecret != "" && p.cachedUntil.After(nowTime.Add(time.Minute)) {
		return p.cachedSecret, nil
	}
	key, err := parseECPrivateKey(p.privateKeyPEM)
	if err != nil {
		return "", err
	}
	now := nowTime.Unix()
	expiresAt := nowTime.Add(30 * 24 * time.Hour)
	header := map[string]string{"alg": "ES256", "kid": p.keyID, "typ": "JWT"}
	payload := map[string]any{
		"iss": p.teamID,
		"iat": now,
		"exp": expiresAt.Unix(),
		"aud": appleIssuer,
		"sub": p.clientID,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("apple client secret header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("apple client secret payload: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("apple client secret sign: %w", err)
	}
	p.cachedSecret = signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	p.cachedUntil = nowTime.Add(10 * time.Minute)
	if p.cachedUntil.After(expiresAt.Add(-time.Minute)) {
		p.cachedUntil = expiresAt.Add(-time.Minute)
	}
	return p.cachedSecret, nil
}

func parseECPrivateKey(raw string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.ReplaceAll(raw, `\n`, "\n")))
	if block == nil {
		return nil, fmt.Errorf("apple private key PEM is invalid")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apple private key parse: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apple private key must be ECDSA")
	}
	return ecKey, nil
}

func postForm(ctx context.Context, client *http.Client, endpoint string, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return
		}
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return tokenResponse{}, err
	}
	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || tokenResp.Error != "" || tokenResp.IDToken == "" {
		return tokenResponse{}, fmt.Errorf("provider token exchange failed")
	}
	return tokenResp, nil
}

func parseAppleUserDisplayName(raw string) string {
	if raw == "" {
		return ""
	}
	var payload struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	if err := decoder.Decode(&payload); err != nil {
		return ""
	}
	displayName := strings.TrimSpace(payload.Name.FirstName + " " + payload.Name.LastName)
	if displayName == "" {
		return ""
	}
	return displayName
}

func oauthHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	KID string `json:"kid"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func verifyIDTokenSignature(ctx context.Context, client *http.Client, idToken, jwksURL string) error {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("%w: id_token has invalid signature shape", domain.ErrOAuthProviderInvalid)
	}
	headerPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("%w: id_token header decode", domain.ErrOAuthProviderInvalid)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerPayload, &header); err != nil {
		return fmt.Errorf("%w: id_token header json", domain.ErrOAuthProviderInvalid)
	}
	if !acceptedIDTokenAlgorithms()[header.Alg] || header.KID == "" {
		return fmt.Errorf("%w: unsupported id_token signature", domain.ErrOAuthProviderInvalid)
	}
	key, err := fetchRSAKey(ctx, client, jwksURL, header.KID)
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("%w: id_token signature decode", domain.ErrOAuthProviderInvalid)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("%w: id_token signature verify", domain.ErrOAuthProviderInvalid)
	}
	return nil
}

func acceptedIDTokenAlgorithms() map[string]bool {
	return map[string]bool{"RS256": true}
}

func fetchRSAKey(ctx context.Context, client *http.Client, jwksURL, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: jwks fetch failed", domain.ErrOAuthProviderInvalid)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("%w: jwks decode", domain.ErrOAuthProviderInvalid)
	}
	for _, key := range jwks.Keys {
		if key.KID == kid && key.KTY == "RSA" {
			return rsaKeyFromJWK(key)
		}
	}
	return nil, fmt.Errorf("%w: jwks key not found", domain.ErrOAuthProviderInvalid)
}

func rsaKeyFromJWK(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("%w: jwks modulus decode", domain.ErrOAuthProviderInvalid)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("%w: jwks exponent decode", domain.ErrOAuthProviderInvalid)
	}
	var e uint64
	switch len(eBytes) {
	case 1, 2, 3, 4:
		var padded [8]byte
		copy(padded[8-len(eBytes):], eBytes)
		e = binary.BigEndian.Uint64(padded[:])
	default:
		return nil, fmt.Errorf("%w: jwks exponent invalid", domain.ErrOAuthProviderInvalid)
	}
	if e > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("%w: jwks exponent overflows int", domain.ErrOAuthProviderInvalid)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e)}, nil
}
