// SPDX-License-Identifier: Apache-2.0

package amm

import (
	"errors"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

func TestNewBesuBreaker_Validation(t *testing.T) {
	t.Parallel()

	signer, err := registry.NewStaticKeySigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	cases := []struct {
		name   string
		cfg    BesuConfig
		signer registry.TransactionSigner
		want   string
	}{
		{
			name:   "missing RPC URL",
			cfg:    BesuConfig{AMMAddress: "0xabc", ChainID: 1337},
			signer: signer,
			want:   "BESU_RPC_URL is required",
		},
		{
			name:   "missing AMM address",
			cfg:    BesuConfig{RPCURL: "http://localhost:8545", ChainID: 1337},
			signer: signer,
			want:   "AMM_ADDRESS is required",
		},
		{
			name:   "nil signer",
			cfg:    BesuConfig{RPCURL: "http://localhost:8545", AMMAddress: "0xabc", ChainID: 1337},
			signer: nil,
			want:   "no transaction signer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewBesuBreaker(tc.cfg, tc.signer)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestErrSentinels(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrNotPaused, ErrNotPaused) || !errors.Is(ErrNoSigner, ErrNoSigner) {
		t.Fatal("sentinel errors must be identity-comparable")
	}
}
