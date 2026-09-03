// SPDX-License-Identifier: Apache-2.0

// Package identity provides an EVM client for the IdentityRegistry contract.
// The client uses an embedded ABI and go-ethereum/ethclient for read-only verification
// of participant registration and governance eligibility (T057).
package identity

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ABIJSON is the minimal ABI for IdentityRegistry view functions.
const ABIJSON = `[
{"type":"function","name":"canTransact","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"canGovern","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"isWhitelisted","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"name":"","type":"bool"}]},
{"type":"function","name":"getParticipant","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"components":[
  {"name":"legalName","type":"string"},
  {"name":"role","type":"uint8"},
  {"name":"status","type":"uint8"},
  {"name":"zkPointer","type":"bytes32"},
  {"name":"certFingerprint","type":"bytes32"},
  {"name":"lastUpdate","type":"uint256"}
],"name":"","type":"tuple"}]},
{"type":"function","name":"getCertFingerprint","stateMutability":"view","inputs":[
  {"name":"account","type":"address"}
],"outputs":[{"name":"","type":"bytes32"}]}
]`

// ParticipantRole mirrors the Solidity enum.
type ParticipantRole uint8

const (
	RoleNone           ParticipantRole = 0
	RoleCentralBank    ParticipantRole = 1
	RoleCommercialBank ParticipantRole = 2
	RoleGovernance     ParticipantRole = 3
)

// KycStatus mirrors the Solidity enum.
type KycStatus uint8

const (
	KycUnverified KycStatus = 0
	KycVerified   KycStatus = 1
	KycSuspended  KycStatus = 2
	KycRevoked    KycStatus = 3
)

// Participant mirrors the on-chain Participant struct returned by getParticipant.
type Participant struct {
	LegalName       string
	Role            ParticipantRole
	Status          KycStatus
	ZKPointer       [32]byte
	CertFingerprint [32]byte
	LastUpdate      *big.Int
}

// Config holds the connection parameters for the IdentityRegistry client.
type Config struct {
	RPCURL          string
	ContractAddress string
	Timeout         time.Duration
}

// Client is the EVM client for the IdentityRegistry contract.
type Client struct {
	contract common.Address
	ec       *ethclient.Client
	abi      abi.ABI
	timeout  time.Duration
}

// NewClient opens an RPC connection and parses the ABI.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ContractAddress == "" {
		return nil, errors.New("ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	parsed, err := evm.ParseABI(ABIJSON)
	if err != nil {
		return nil, fmt.Errorf("parse IdentityRegistry ABI: %w", err)
	}
	return &Client{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		abi:      parsed,
		timeout:  cfg.Timeout,
	}, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// IsRegistered checks whether a bank address is registered (canTransact) in the IdentityRegistry.
func (c *Client) IsRegistered(ctx context.Context, address string) (bool, error) {
	var result bool
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "canTransact",
		[]interface{}{common.HexToAddress(address)}, &result); err != nil {
		return false, err
	}
	return result, nil
}

// CanGovern checks if an address holds a governance-eligible role.
func (c *Client) CanGovern(ctx context.Context, address string) (bool, error) {
	var result bool
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "canGovern",
		[]interface{}{common.HexToAddress(address)}, &result); err != nil {
		return false, err
	}
	return result, nil
}

// IsWhitelisted checks if an address has Verified KYC status.
func (c *Client) IsWhitelisted(ctx context.Context, address string) (bool, error) {
	var result bool
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "isWhitelisted",
		[]interface{}{common.HexToAddress(address)}, &result); err != nil {
		return false, err
	}
	return result, nil
}

// GetCertFingerprint returns the X.509 certificate fingerprint for an address.
func (c *Client) GetCertFingerprint(ctx context.Context, address string) ([32]byte, error) {
	var result [32]byte
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "getCertFingerprint",
		[]interface{}{common.HexToAddress(address)}, &result); err != nil {
		return [32]byte{}, err
	}
	return result, nil
}
