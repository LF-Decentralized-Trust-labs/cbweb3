// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc/codes"
)

// TestRegisterParticipantOnChain covers the wallet guard and the idempotent
// already-registered path (the Noop client reports CanTransact=true).
func TestRegisterParticipantOnChain(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)

	if _, err := client.RegisterParticipantOnChain(ctx, &compliancv1.RegisterParticipantOnChainRequest{}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("empty wallet: want InvalidArgument, got %v", err)
	}

	resp, err := client.RegisterParticipantOnChain(ctx, &compliancv1.RegisterParticipantOnChainRequest{
		WalletAddress:   "0xabc",
		InstitutionName: "Bank A",
		BankCode:        "AAA",
	})
	if err != nil {
		t.Fatalf("valid wallet: unexpected error %v", err)
	}
	if !resp.AlreadyRegistered {
		t.Error("want AlreadyRegistered=true (Noop CanTransact reports true)")
	}
}

// TestRegisterCurrencyOnChain covers the argument guards and the Unimplemented
// degradation when no on-chain currency registrar is wired (Noop client).
func TestRegisterCurrencyOnChain(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)

	if _, err := client.RegisterCurrencyOnChain(ctx, &compliancv1.RegisterCurrencyOnChainRequest{}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("empty cb_address: want InvalidArgument, got %v", err)
	}
	if _, err := client.RegisterCurrencyOnChain(ctx, &compliancv1.RegisterCurrencyOnChainRequest{CbAddress: "0xcb"}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("empty currency: want InvalidArgument, got %v", err)
	}
	if _, err := client.RegisterCurrencyOnChain(ctx, &compliancv1.RegisterCurrencyOnChainRequest{
		CbAddress: "0xcb", Currency: "BRL",
	}); codeOf(err) != codes.Unimplemented {
		t.Fatalf("noop currency registrar: want Unimplemented, got %v", err)
	}
}

// TestRegisterPairOnChain covers the argument guards and the Unimplemented
// degradation when no on-chain pair registrar is wired (Noop client).
func TestRegisterPairOnChain(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)

	if _, err := client.RegisterPairOnChain(ctx, &compliancv1.RegisterPairOnChainRequest{}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("empty currencies: want InvalidArgument, got %v", err)
	}
	if _, err := client.RegisterPairOnChain(ctx, &compliancv1.RegisterPairOnChainRequest{
		CurrencyA: "BRL", CurrencyB: "ARS",
	}); codeOf(err) != codes.Unimplemented {
		t.Fatalf("noop pair registrar: want Unimplemented, got %v", err)
	}
}
