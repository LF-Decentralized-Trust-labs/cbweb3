// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// Production key custody never exports the private key: a KMS signs a digest and returns a signature.
// So this package must be able to sign a transaction WITHOUT holding the key, and the interface it
// needs is deliberately the same shape scenario A's toolkit already uses (Sign over a 32-byte digest,
// GetPublicKey for the address) — the two scenarios are expected to merge, so a second design would
// be work to undo.
//
// The V byte is where this goes wrong quietly. secp256k1 recovery ids are produced as 27/28 by some
// stacks and go-ethereum expects 0/1 at index 64: get it wrong and the signature is well-formed,
// type-checks, and is rejected by the node with an unhelpful error. Scenario A left a comment about
// exactly this. These tests pin it.

// fakeProvider signs with a real key but through the remote interface, so the transport shape is
// exercised without a KMS.
type fakeProvider struct {
	key       *ecdsa.PrivateKey
	vOffset   byte // added to V, to emulate a provider that returns 27/28
	signCalls int
	err       error
}

func (f *fakeProvider) Sign(_ context.Context, _ string, digest []byte) ([]byte, error) {
	f.signCalls++
	if f.err != nil {
		return nil, f.err
	}
	sig, err := gethcrypto.Sign(digest, f.key)
	if err != nil {
		return nil, err
	}
	sig[64] += f.vOffset
	return sig, nil
}

func (f *fakeProvider) GetPublicKey(_ context.Context, _ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return gethcrypto.FromECDSAPub(&f.key.PublicKey), nil
}

func newFakeProvider(t *testing.T, vOffset byte) *fakeProvider {
	t.Helper()
	k, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return &fakeProvider{key: k, vOffset: vOffset}
}

// The address must come from the provider's public key: with a KMS there is no key to derive it from
// locally, and an address that disagrees with the signer is an account that cannot sign.
func TestNewRemoteSigner_AddressComesFromTheProvider(t *testing.T) {
	p := newFakeProvider(t, 0)
	s, err := NewRemoteSigner(context.Background(), p, "spoke-brl/hub-gateway", big.NewInt(1337))
	if err != nil {
		t.Fatalf("remote signer: %v", err)
	}
	want := gethcrypto.PubkeyToAddress(p.key.PublicKey)
	if s.Address() != want {
		t.Fatalf("address = %s, want %s", s.Address().Hex(), want.Hex())
	}
}

// A transaction signed remotely must verify to the same address — which is what proves the V byte and
// the digest are right, not merely that bytes came back.
func TestRemoteSigner_SignsATransactionRecoverably(t *testing.T) {
	for name, vOffset := range map[string]byte{
		"provider returns v as 0/1":   0,
		"provider returns v as 27/28": 27,
	} {
		t.Run(name, func(t *testing.T) {
			p := newFakeProvider(t, vOffset)
			s, err := NewRemoteSigner(context.Background(), p, "k", big.NewInt(1337))
			if err != nil {
				t.Fatalf("remote signer: %v", err)
			}
			tx := types.NewTransaction(0, s.Address(), big.NewInt(0), 21000, big.NewInt(1), nil)
			signed, err := s.signTx(context.Background(), tx)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			london := types.NewLondonSigner(big.NewInt(1337))
			from, err := types.Sender(london, signed)
			if err != nil {
				t.Fatalf("recover sender: %v", err)
			}
			if from != s.Address() {
				t.Fatalf("recovered %s, want %s — the V byte or the digest is wrong", from.Hex(), s.Address().Hex())
			}
		})
	}
}

// A local signer must keep signing exactly as before: the remote path is additive.
func TestLocalSigner_StillSignsRecoverably(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	tx := types.NewTransaction(0, s.Address(), big.NewInt(0), 21000, big.NewInt(1), nil)
	signed, err := s.signTx(context.Background(), tx)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	from, err := types.Sender(types.NewLondonSigner(big.NewInt(1337)), signed)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if from != s.Address() {
		t.Fatalf("recovered %s, want %s", from.Hex(), s.Address().Hex())
	}
}

// A provider that cannot produce the public key must fail at construction, not later at the first
// transaction: an unusable signer discovered mid-payment is far worse than one refused at wiring.
func TestNewRemoteSigner_FailsFastOnAnUnreachableProvider(t *testing.T) {
	p := newFakeProvider(t, 0)
	p.err = errors.New("kms unavailable")
	if _, err := NewRemoteSigner(context.Background(), p, "k", big.NewInt(1337)); err == nil {
		t.Fatal("expected construction to fail when the provider cannot answer")
	}
}

// A remote signer holds no key material: that is the whole point, and the type must not offer one.
func TestRemoteSigner_HoldsNoPrivateKey(t *testing.T) {
	p := newFakeProvider(t, 0)
	s, err := NewRemoteSigner(context.Background(), p, "k", big.NewInt(1337))
	if err != nil {
		t.Fatalf("remote signer: %v", err)
	}
	if s.key != nil {
		t.Fatal("a remote signer must not hold a private key")
	}
}
