package providers

import (
	"fmt"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kmsprovider"
)

func New(provider string) (kmsprovider.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", ProviderLocalKMS:
		return NewLocalKMS(), nil
	case ProviderAWSKMS, ProviderGCPKMS:
		return nil, fmt.Errorf("kms provider not implemented yet: %s", provider)
	default:
		return nil, fmt.Errorf("unsupported kms provider: %s", provider)
	}
}
