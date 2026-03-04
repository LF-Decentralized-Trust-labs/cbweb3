// This file tests provider and validator fallback chain behavior.
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

type failingProvider struct{}

func (f failingProvider) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, errors.New("fail")
}

type successProvider struct{}

func (s successProvider) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{AccessToken: "ok", ExpiresIn: 1, TokenType: "Bearer"}, nil
}

type failingValidator struct{}

func (f failingValidator) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{}, errors.New("invalid")
}

type successValidator struct{}

func (s successValidator) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{Subject: "bank-a"}, nil
}

func TestProviderChain(t *testing.T) {
	t.Parallel()
	chain := NewProviderChain(failingProvider{}, successProvider{})
	token, err := chain.Authenticate(context.Background(), "id", "secret")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if token.AccessToken != "ok" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestValidatorChain(t *testing.T) {
	t.Parallel()
	chain := NewValidatorChain(failingValidator{}, successValidator{})
	claims, err := chain.Validate(context.Background(), "token")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if claims.Subject != "bank-a" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
}

func TestProviderChainWithoutProviders(t *testing.T) {
	t.Parallel()
	chain := NewProviderChain()
	if _, err := chain.Authenticate(context.Background(), "id", "secret"); err == nil {
		t.Fatal("expected error with empty chain")
	}
}

func TestValidatorChainWithoutValidators(t *testing.T) {
	t.Parallel()
	chain := NewValidatorChain()
	if _, err := chain.Validate(context.Background(), "token"); err == nil {
		t.Fatal("expected error with empty chain")
	}
}

