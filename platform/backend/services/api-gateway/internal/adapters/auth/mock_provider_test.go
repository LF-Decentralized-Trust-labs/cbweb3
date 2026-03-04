// This file tests mock provider credential checks and token generation.
package auth

import (
	"context"
	"testing"
	"time"
)

func TestMockProviderAndValidator(t *testing.T) {
	t.Parallel()

	provider, err := NewMockAuthProvider(
		map[string]string{"bank-a": "secret-a"},
		"issuer-test",
		"aud-test",
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("unexpected error creating mock provider: %v", err)
	}

	token, err := provider.Authenticate(context.Background(), "bank-a", "secret-a")
	if err != nil {
		t.Fatalf("expected valid credentials, got error: %v", err)
	}
	if token.TokenType != "Bearer" || token.AccessToken == "" {
		t.Fatalf("unexpected token response: %+v", token)
	}

	validator := NewMockTokenValidator(provider.PublicKey(), "issuer-test", "aud-test")
	claims, err := validator.Validate(context.Background(), token.AccessToken)
	if err != nil {
		t.Fatalf("expected token validation to pass, got: %v", err)
	}
	if claims.Subject != "bank-a" {
		t.Fatalf("expected subject bank-a, got %s", claims.Subject)
	}
}

func TestMockProviderInvalidCredentials(t *testing.T) {
	t.Parallel()

	provider, err := NewMockAuthProvider(
		map[string]string{"bank-a": "secret-a"},
		"issuer-test",
		"aud-test",
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("unexpected error creating mock provider: %v", err)
	}

	if _, err := provider.Authenticate(context.Background(), "bank-a", "wrong"); err == nil {
		t.Fatal("expected error with wrong credentials")
	}
}

func TestMockProviderSignWalletBindMessage(t *testing.T) {
	t.Parallel()

	provider, err := NewMockAuthProvider(
		map[string]string{"bank-a": "secret-a"},
		"issuer-test",
		"aud-test",
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("unexpected error creating mock provider: %v", err)
	}

	msg, err := provider.SignWalletBindMessage("bank-a", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == "" {
		t.Fatal("expected a non-empty message")
	}

	if _, err := provider.SignWalletBindMessage("", ""); err == nil {
		t.Fatal("expected validation error")
	}
}

