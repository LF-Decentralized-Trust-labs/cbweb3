package registry

import "context"

// NoopRegistryClient is a no-op implementation used in local/test environments
// where no Besu node is available. All write operations succeed silently;
// reads return safe defaults (authorized=true, role=0).
type NoopRegistryClient struct{}

func (NoopRegistryClient) SetParticipant(_ context.Context, _, _ string, _ bool) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopRegistryClient) SetCertFingerprint(_ context.Context, _ string, _ [32]byte) (string, error) {
	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (NoopRegistryClient) IsMemberAuthorized(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (NoopRegistryClient) GetMemberRole(_ context.Context, _ string) (uint8, error) {
	return 0, nil
}

func (NoopRegistryClient) GetCertFingerprint(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}
