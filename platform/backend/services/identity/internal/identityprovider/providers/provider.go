package providers

import "time"

const (
	ProviderLocal      = "local"
	ProviderDWalletAPI = "dwallet_api"
)

// FactoryConfig contains provider selection and endpoint settings.
type FactoryConfig struct {
	Provider       string
	HostURL        string
	JWTSecret      string
	AccessTokenTTL time.Duration
	RequestTimeout time.Duration
}
