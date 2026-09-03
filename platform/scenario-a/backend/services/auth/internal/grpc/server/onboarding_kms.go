// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateOnboardingKey creates a new secp256k1 key in the KMS for the given bank.
func (s *identityService) CreateOnboardingKey(ctx context.Context, req *authv1.CreateOnboardingKeyRequest) (*authv1.CreateOnboardingKeyResponse, error) {
	if req.BankCode == "" {
		return nil, status.Error(codes.InvalidArgument, "bank_code is required")
	}

	keyInfo, err := s.kms.CreateKey(ctx, req.BankCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "kms create key: %v", err)
	}

	pubHex, err := s.addressToPubKeyHex(ctx, req.BankCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "recover pub key: %v", err)
	}

	return &authv1.CreateOnboardingKeyResponse{
		PubKeyHex: pubHex,
		Address:   keyInfo.Address,
	}, nil
}

// SignOnboardingPoP signs a PoP nonce with the KMS key.
func (s *identityService) SignOnboardingPoP(ctx context.Context, req *authv1.SignOnboardingPoPRequest) (*authv1.SignOnboardingPoPResponse, error) {
	if req.KeyId == "" || req.NonceHex == "" {
		return nil, status.Error(codes.InvalidArgument, "key_id and nonce_hex are required")
	}

	nonceBytes, err := hex.DecodeString(req.NonceHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid nonce hex: %v", err)
	}

	digest := sha256.Sum256(nonceBytes)
	digestHex := hex.EncodeToString(digest[:])

	result, err := s.kms.Sign(ctx, req.KeyId, digestHex)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "kms sign: %v", err)
	}

	// result.Signature is "0x{R 32B}{S 32B}{V 1B}" where V may be 0/1 or 27/28.
	sigHex := result.Signature
	if len(sigHex) >= 2 && sigHex[:2] == "0x" {
		sigHex = sigHex[2:]
	}

	// Normalize V: cast-style (27/28) -> 0/1
	if len(sigHex) == 130 {
		vByte := sigHex[128:130]
		switch vByte {
		case "1b":
			sigHex = sigHex[:128] + "00"
		case "1c":
			sigHex = sigHex[:128] + "01"
		}
	}

	pubHex, err := s.addressToPubKeyHex(ctx, req.KeyId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "recover pub key: %v", err)
	}

	return &authv1.SignOnboardingPoPResponse{
		SignatureHex: sigHex,
		PubKeyHex:    pubHex,
	}, nil
}

// GetOnboardingKey returns the address and public key for an existing KMS key.
func (s *identityService) GetOnboardingKey(ctx context.Context, req *authv1.GetOnboardingKeyRequest) (*authv1.GetOnboardingKeyResponse, error) {
	if req.KeyId == "" {
		return nil, status.Error(codes.InvalidArgument, "key_id is required")
	}

	addr, err := s.kms.GetAddress(ctx, req.KeyId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "key not found: %v", err)
	}

	pubHex, err := s.addressToPubKeyHex(ctx, req.KeyId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "recover pub key: %v", err)
	}

	return &authv1.GetOnboardingKeyResponse{
		PubKeyHex: pubHex,
		Address:   addr,
	}, nil
}

// addressToPubKeyHex recovers the uncompressed public key (04...) for a KMS key
// by signing a known digest and using ecrecover.
func (s *identityService) addressToPubKeyHex(ctx context.Context, keyID string) (string, error) {
	knownData := sha256.Sum256([]byte("cbweb3-pub-key-recovery"))
	digestHex := hex.EncodeToString(knownData[:])

	result, err := s.kms.Sign(ctx, keyID, digestHex)
	if err != nil {
		return "", fmt.Errorf("sign for recovery: %w", err)
	}

	sigHex := result.Signature
	if len(sigHex) >= 2 && sigHex[:2] == "0x" {
		sigHex = sigHex[2:]
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", fmt.Errorf("decode sig hex: %w", err)
	}

	if len(sigBytes) != 65 {
		return "", fmt.Errorf("unexpected sig length %d", len(sigBytes))
	}

	// Normalize V for go-ethereum: expects 0 or 1
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	pub, err := gethcrypto.SigToPub(knownData[:], sigBytes)
	if err != nil {
		return "", fmt.Errorf("ecrecover: %w", err)
	}

	uncompressed := gethcrypto.FromECDSAPub(pub)
	return hex.EncodeToString(uncompressed), nil
}
