// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// Opening a corridor is bilateral: PairRegistry admits only getCentralBankOf(tokenB) as the
// confirmer. Since each currency's issuance authority moved to its own central bank, a CB
// confirming a corridor it does not own is an ordinary operator mistake — and the on-chain
// revert reaches the caller with no decoded reason, as a generic "transaction reverted — check
// contract permissions and token allowances" plus a wasted transaction. These tests pin the
// pre-check that refuses it with the real reason instead.

type fakeTokenAuthority struct {
	self      string
	cbOfToken map[string]string
	err       error
	calls     int
}

func (f *fakeTokenAuthority) CentralBankOfToken(_ context.Context, token string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.cbOfToken[token], nil
}

func (f *fakeTokenAuthority) HubSignerAddress() string { return f.self }

const (
	cbOfTokenB  = "0xBBBB000000000000000000000000000000000001"
	someOtherCB = "0xAAAA000000000000000000000000000000000002"
)

func pairOnChain() []domain.PairEntry {
	return []domain.PairEntry{{PairID: "W-BRL-W-ARS", TokenA: "0xTokenA", TokenB: "0xTokenB"}}
}

func TestConfirmPair_RefusesWhenNotTheCentralBankOfTokenB(t *testing.T) {
	client := &fakePairClient{confirmTx: "0xshouldnothappen", active: pairOnChain()}
	svc := NewPairService(client, &fakePairRepo{}).WithTokenAuthorityReader(&fakeTokenAuthority{
		self:      someOtherCB,
		cbOfToken: map[string]string{"0xTokenB": cbOfTokenB},
	})

	_, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "W-BRL-W-ARS"})

	if !errors.Is(err, ErrNotCentralBankOfTokenB) {
		t.Fatalf("expected ErrNotCentralBankOfTokenB, got %v", err)
	}
	// The point of the pre-check: no transaction is spent on a certain revert.
	if client.confirmCalls != 0 {
		t.Fatalf("expected no confirmPair transaction, got %d", client.confirmCalls)
	}
	// The message must name who may confirm — that is what the generic revert never said.
	if got := err.Error(); !contains(got, cbOfTokenB) || !contains(got, someOtherCB) {
		t.Fatalf("error should name both the expected CB and this gateway, got %q", got)
	}
}

func TestConfirmPair_ProceedsForTheCentralBankOfTokenB(t *testing.T) {
	client := &fakePairClient{confirmTx: "0xconfirmed", active: pairOnChain()}
	svc := NewPairService(client, &fakePairRepo{}).WithTokenAuthorityReader(&fakeTokenAuthority{
		self:      cbOfTokenB,
		cbOfToken: map[string]string{"0xTokenB": cbOfTokenB},
	})

	res, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "W-BRL-W-ARS"})
	if err != nil {
		t.Fatalf("the issuing CB of token B must be allowed to confirm: %v", err)
	}
	if res.TxHash != "0xconfirmed" || client.confirmCalls != 1 {
		t.Fatalf("expected exactly one confirmPair, got tx=%q calls=%d", res.TxHash, client.confirmCalls)
	}
}

// A failed read must not block a legitimate confirm: the pre-check is an improvement on the
// error path, not a new gate. Anything it cannot resolve falls through to the chain, which
// remains the authority.
func TestConfirmPair_FallsThroughWhenAuthorityCannotBeResolved(t *testing.T) {
	cases := map[string]*fakeTokenAuthority{
		"reader error":    {self: someOtherCB, err: errors.New("rpc down")},
		"unknown signer":  {self: "", cbOfToken: map[string]string{"0xTokenB": cbOfTokenB}},
		"empty cb answer": {self: someOtherCB, cbOfToken: map[string]string{}},
		"zero cb address": {self: someOtherCB, cbOfToken: map[string]string{
			"0xTokenB": "0x0000000000000000000000000000000000000000",
		}},
	}
	for name, auth := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakePairClient{confirmTx: "0xconfirmed", active: pairOnChain()}
			svc := NewPairService(client, &fakePairRepo{}).WithTokenAuthorityReader(auth)

			_, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "W-BRL-W-ARS"})
			if errors.Is(err, ErrNotCentralBankOfTokenB) {
				t.Fatalf("an unresolvable authority must not be reported as a mismatch")
			}
			if client.confirmCalls != 1 {
				t.Fatalf("expected the on-chain attempt to still run, got %d calls", client.confirmCalls)
			}
		})
	}
}

// An unknown pair id is left to the chain, which reports it as not found.
func TestConfirmPair_FallsThroughWhenPairAbsentOnChain(t *testing.T) {
	client := &fakePairClient{confirmTx: "0xconfirmed", active: []domain.PairEntry{}}
	auth := &fakeTokenAuthority{self: someOtherCB, cbOfToken: map[string]string{"0xTokenB": cbOfTokenB}}
	svc := NewPairService(client, &fakePairRepo{}).WithTokenAuthorityReader(auth)

	if _, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "W-BRL-W-ARS"}); err != nil {
		t.Fatalf("unexpected refusal for an absent pair: %v", err)
	}
	if auth.calls != 0 {
		t.Fatalf("token authority must not be consulted for a pair that is not on-chain")
	}
}

// Without the reader wired, behaviour is exactly as before: the chain decides.
func TestConfirmPair_WithoutAuthorityReaderBehavesAsBefore(t *testing.T) {
	client := &fakePairClient{confirmTx: "0xconfirmed", active: pairOnChain()}
	svc := NewPairService(client, &fakePairRepo{})

	if _, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "W-BRL-W-ARS"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.confirmCalls != 1 {
		t.Fatalf("expected the on-chain confirm to run, got %d calls", client.confirmCalls)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
