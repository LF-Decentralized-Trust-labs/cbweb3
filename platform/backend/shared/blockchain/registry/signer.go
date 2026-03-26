package registry

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// ErrNoSigner is returned when a write operation is attempted on a BesuClient
// that has no TransactionSigner configured.
var ErrNoSigner = errors.New("registry: no transaction signer configured")

// TransactionSigner abstracts the key material needed to sign Ethereum
// transactions. Central Banks use StaticKeySigner (from CB_PRIVATE_KEY);
// Commercial Banks use a KMS-backed signer loaded at runtime.
type TransactionSigner interface {
	SignerAddress(ctx context.Context) (string, error)
	SignTx(ctx context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
}

// StaticKeySigner implements TransactionSigner with a fixed secp256k1 private
// key. Used by the Central Bank where the key is known at startup via
// CB_PRIVATE_KEY.
type StaticKeySigner struct {
	privKey *ecdsa.PrivateKey
}

// NewStaticKeySigner parses a hex-encoded secp256k1 private key (with or
// without 0x prefix) and returns a StaticKeySigner.
func NewStaticKeySigner(privKeyHex string) (*StaticKeySigner, error) {
	cleaned := strings.TrimSpace(strings.TrimPrefix(privKeyHex, "0x"))
	if cleaned == "" {
		return nil, errors.New("registry: private key hex is empty")
	}
	b, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("registry: decoding private key: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(b)
	if err != nil {
		return nil, fmt.Errorf("registry: parsing private key: %w", err)
	}
	return &StaticKeySigner{privKey: privKey}, nil
}

func (s *StaticKeySigner) SignerAddress(_ context.Context) (string, error) {
	return gethcrypto.PubkeyToAddress(s.privKey.PublicKey).Hex(), nil
}

func (s *StaticKeySigner) SignTx(_ context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	ethSigner := types.NewEIP155Signer(chainID)
	return types.SignTx(tx, ethSigner, s.privKey)
}
