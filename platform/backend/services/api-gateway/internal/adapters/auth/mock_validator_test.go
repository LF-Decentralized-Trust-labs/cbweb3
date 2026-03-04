// This file tests mock JWT validator edge cases and claim validation rules.
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestMockTokenValidatorInvalidCases(t *testing.T) {
	t.Parallel()

	provider, err := NewMockAuthProvider(
		map[string]string{"bank-a": "secret-a"},
		"issuer-test",
		"aud-test",
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}
	token, err := provider.Authenticate(context.Background(), "bank-a", "secret-a")
	if err != nil {
		t.Fatalf("unexpected auth error: %v", err)
	}

	t.Run("invalid issuer", func(t *testing.T) {
		validator := NewMockTokenValidator(provider.PublicKey(), "wrong-issuer", "aud-test")
		if _, err := validator.Validate(context.Background(), token.AccessToken); err == nil {
			t.Fatal("expected issuer validation error")
		}
	})

	t.Run("invalid audience", func(t *testing.T) {
		validator := NewMockTokenValidator(provider.PublicKey(), "issuer-test", "wrong-audience")
		if _, err := validator.Validate(context.Background(), token.AccessToken); err == nil {
			t.Fatal("expected audience validation error")
		}
	})

	t.Run("wrong algorithm", func(t *testing.T) {
		now := time.Now()
		claims := mockJWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "bank-a",
				Issuer:    "issuer-test",
				Audience:  jwt.ClaimStrings{"aud-test"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
				IssuedAt:  jwt.NewNumericDate(now),
			},
		}
		hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		raw, err := hsToken.SignedString([]byte("secret"))
		if err != nil {
			t.Fatalf("failed to sign hs token: %v", err)
		}

		validator := NewMockTokenValidator(provider.PublicKey(), "issuer-test", "aud-test")
		if _, err := validator.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected algorithm validation error")
		}
	})
}

