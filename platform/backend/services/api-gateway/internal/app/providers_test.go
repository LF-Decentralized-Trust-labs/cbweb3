// This file tests auth provider selection based on configured auth mode.
package app

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

type providerStub struct{}

func (p providerStub) Authenticate(_ context.Context, _, _ string) (domain.AuthToken, error) {
	return domain.AuthToken{}, nil
}

type validatorStub struct{}

func (v validatorStub) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{}, nil
}

func TestChooseProvidersModes(t *testing.T) {
	t.Parallel()

	_, _, err := chooseProviders(config.ModeMock, providerStub{}, providerStub{}, validatorStub{}, validatorStub{})
	if err != nil {
		t.Fatalf("mock mode should be valid: %v", err)
	}

	_, _, err = chooseProviders(config.ModeKeycloak, providerStub{}, providerStub{}, validatorStub{}, validatorStub{})
	if err != nil {
		t.Fatalf("keycloak mode should be valid: %v", err)
	}

	_, _, err = chooseProviders(config.ModeHybrid, providerStub{}, providerStub{}, validatorStub{}, validatorStub{})
	if err != nil {
		t.Fatalf("hybrid mode should be valid: %v", err)
	}
}

func TestChooseProvidersInvalidMode(t *testing.T) {
	t.Parallel()

	_, _, err := chooseProviders("invalid", providerStub{}, providerStub{}, validatorStub{}, validatorStub{})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

