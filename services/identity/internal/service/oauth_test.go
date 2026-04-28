package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"sandbox_url_shortener/services/identity/internal/domain"
)

func TestOAuthServiceStartStoresPKCEAndNonce(t *testing.T) {
	repo := newFakeRepo()
	provider := fakeProvider{name: domain.ProviderGoogle}
	authSvc := NewAuthService(repo, slog.Default(), 30*24*time.Hour, 7*24*time.Hour)
	oauthSvc := NewOAuthService(repo, authSvc, provider)

	start, err := oauthSvc.Start(context.Background(), domain.ProviderGoogle, "http://web.test/auth/callback", time.Now())
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if start.State == "" {
		t.Fatal("expected state")
	}
	if !strings.Contains(start.RedirectURL, "code_challenge=") || !strings.Contains(start.RedirectURL, "nonce=") {
		t.Fatalf("redirect url missing PKCE or nonce: %s", start.RedirectURL)
	}
	if repo.oauthState == nil || repo.oauthState.PKCEVerifierHash == "" || repo.oauthState.NonceHash == "" {
		t.Fatalf("oauth state not persisted: %#v", repo.oauthState)
	}
}

func TestOAuthServiceCompleteIssuesSession(t *testing.T) {
	repo := newFakeRepo()
	provider := fakeProvider{name: domain.ProviderGoogle}
	authSvc := NewAuthService(repo, slog.Default(), 30*24*time.Hour, 7*24*time.Hour)
	oauthSvc := NewOAuthService(repo, authSvc, provider)

	start, err := oauthSvc.Start(context.Background(), domain.ProviderGoogle, "http://web.test/auth/callback", time.Now())
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	issued, redirectURL, err := oauthSvc.Complete(context.Background(), domain.ProviderGoogle, start.State, "code-1", start.PKCEVerifier, "", "", "127.0.0.1", "test-agent", time.Now())
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if redirectURL != "http://web.test/auth/callback" {
		t.Fatalf("redirectURL = %q", redirectURL)
	}
	if issued.Token == "" || issued.Account.Email != "oauth@example.com" {
		t.Fatalf("issued = %#v", issued)
	}
}

type fakeProvider struct {
	name string
}

func (p fakeProvider) Name() string {
	return p.name
}

func (p fakeProvider) AuthURL(state, codeChallenge, nonce string) (string, error) {
	return "https://provider.test/auth?state=" + state + "&code_challenge=" + codeChallenge + "&nonce=" + nonce, nil
}

func (p fakeProvider) Exchange(context.Context, string, string, string, string) (domain.OAuthClaims, error) {
	return domain.OAuthClaims{
		Provider:        p.name,
		ProviderSubject: "subject-1",
		Email:           "oauth@example.com",
		EmailVerified:   true,
	}, nil
}
