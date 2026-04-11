package main

import (
	"context"
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var errNoPaladin = errors.New("PALADIN_URL not configured — set it to enable Zeto operations")

// noopZeto is a placeholder ZetoOperator when Paladin is not configured.
type noopZeto struct{}

func (noopZeto) Mint(_ context.Context, _, _ string) (string, error)     { return "", errNoPaladin }
func (noopZeto) Transfer(_ context.Context, _, _ string) (string, error) { return "", errNoPaladin }
func (noopZeto) Lock(_ context.Context, _, _ string) (*ports.ZetoLockResult, error) {
	return nil, errNoPaladin
}
func (noopZeto) Unlock(_ context.Context, _ string) (string, error) { return "", errNoPaladin }
func (noopZeto) TransferLocked(_ context.Context, _, _, _ string) (string, error) {
	return "", errNoPaladin
}
func (noopZeto) Balance(_ context.Context) (string, error) { return "", errNoPaladin }
func (noopZeto) ResolveIdentity(_ context.Context, _ string) (string, error) {
	return "", errNoPaladin
}
