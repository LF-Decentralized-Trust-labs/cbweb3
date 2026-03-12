package providers

import (
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
)

// NewIdentityProvider creates a provider using environment-driven selection.
func NewIdentityProvider(cfg FactoryConfig) (identityprovider.Provider, error) {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	switch cfg.Provider {
	case ProviderLocal:
		return NewLocalProvider(LocalConfig{
			JWTSecret:     cfg.JWTSecret,
			AccessTokenTT: cfg.AccessTokenTTL,
		}), nil
	case ProviderDWalletAPI:
		// Keep the provider contract generic, while this implementation targets D-Wallet API.
		return NewDWalletAPIProvider(cfg.HostURL, cfg.JWTSecret, timeout), nil
	default:
		return nil, fmt.Errorf("unsupported identity provider: %s", cfg.Provider)
	}
}
