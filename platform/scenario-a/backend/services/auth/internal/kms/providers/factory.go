package kmsproviders

import (
	"fmt"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
)

const (
	ProviderLocal  = "local"
	ProviderAws    = "aws"
	ProviderAzure  = "azure"
)

// Config holds the KMS provider selection configuration.
type Config struct {
	Provider string
}

// New instantiates the KMS provider specified by cfg.Provider.
func New(cfg Config) (kms.Provider, error) {
	switch cfg.Provider {
	case ProviderLocal:
		return NewKMSLocal(), nil
	default:
		return nil, fmt.Errorf("unknown KMS provider: %s", cfg.Provider)
	}
}
