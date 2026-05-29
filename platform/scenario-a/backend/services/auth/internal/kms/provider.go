package kms

import (
	"context"
	"errors"
)

// ErrKeyNotFound is returned when no key exists for the requested user.
var ErrKeyNotFound = errors.New("kms: key not found for user")

// ErrNotImplemented is returned by stub providers that have not been implemented yet.
var ErrNotImplemented = errors.New("kms: provider not implemented")

// KeyInfo holds the public identity derived from a generated key.
type KeyInfo struct {
	UserID  string `json:"user_id"`
	Address string `json:"address"` // EVM "0x..." address
	DID     string `json:"did"`     // "did:lac:openprotest:<lowercase address>"
}

// SignResult holds the output of a signing operation.
type SignResult struct {
	Signature string `json:"signature"` // "0x..." hex
	Address   string `json:"address"`   // signer address
}

// Provider is the interface for KMS backends.
type Provider interface {
	Name() string
	CreateKey(ctx context.Context, userID string) (KeyInfo, error)
	Sign(ctx context.Context, userID, digestHex string) (SignResult, error)
	GetAddress(ctx context.Context, userID string) (string, error)
	DeleteKey(ctx context.Context, userID string) error
}
