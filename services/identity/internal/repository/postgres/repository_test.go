package postgres

import (
	"errors"
	"testing"

	"sandbox_url_shortener/services/identity/internal/domain"
)

func TestValidateExistingOAuthLinkingAllowsUnverifiedExistingAccount(t *testing.T) {
	claims := domain.OAuthClaims{
		EmailVerified: true,
	}

	if err := validateExistingOAuthLinking(claims, &domain.Account{EmailVerifiedAt: nil}); err != nil {
		t.Fatalf("unverified existing account err = %v", err)
	}
	if err := validateExistingOAuthLinking(domain.OAuthClaims{EmailVerified: true, PrivateRelay: true}, &domain.Account{}); !errors.Is(err, domain.ErrOAuthLinkingBlocked) {
		t.Fatalf("private relay existing account err = %v", err)
	}
}

func TestNormalizeEmailMatchesDatabaseExpression(t *testing.T) {
	if got := NormalizeEmail(" User@Example.COM "); got != "user@example.com" {
		t.Fatalf("NormalizeEmail = %q", got)
	}
	if got := NormalizeEmail("\tUser@Example.COM\t"); got != "\tuser@example.com\t" {
		t.Fatalf("NormalizeEmail tabs = %q", got)
	}
}
