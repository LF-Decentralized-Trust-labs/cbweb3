// Package services provides CurrencyService — business logic for hub currency registry operations.
// Bridges CurrencyHandler requests to the on-chain CurrencyRegistryClient (006-hub-currency-registry).
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ── Shared request/response types ────────────────────────────────────────────

// CurrencyRegisterRequest carries the input for POST /api/v2/hub/currencies.
type CurrencyRegisterRequest struct {
	Symbol       string `json:"symbol"`
	CountryName  string `json:"country_name"`
	TokenAddress string `json:"token_address"`
	ProposerCB   string `json:"proposer_cb"`
}

// CurrencyRegisterResult is the response body for a successful register.
type CurrencyRegisterResult struct {
	Symbol string `json:"symbol"`
	TxHash string `json:"tx_hash"`
}

// CurrencyRemoveRequest carries the input for DELETE /api/v2/hub/currencies/:symbol.
type CurrencyRemoveRequest struct {
	Symbol string `json:"symbol"`
}

// CurrencyRemoveResult is the response body for a successful remove.
type CurrencyRemoveResult struct {
	Symbol string `json:"symbol"`
	TxHash string `json:"tx_hash"`
}

// CurrencyServiceIface is the interface consumed by currency_handler.go.
type CurrencyServiceIface interface {
	RegisterCurrency(ctx context.Context, req CurrencyRegisterRequest) (*CurrencyRegisterResult, error)
	RemoveCurrency(ctx context.Context, req CurrencyRemoveRequest) (*CurrencyRemoveResult, error)
	ListCurrencies(ctx context.Context) ([]domain.CurrencyEntry, error)
}

// ── Error sentinels ───────────────────────────────────────────────────────────

var ErrCurrencyAlreadyExists = errors.New("CURRENCY_ALREADY_EXISTS")
var ErrTokenAlreadyRegistered = errors.New("TOKEN_ALREADY_REGISTERED")
var ErrCurrencyNotFound = errors.New("CURRENCY_NOT_FOUND")
var ErrCurrencyUnauthorized = errors.New("CURRENCY_UNAUTHORIZED")

// ── CurrencyRegistryClientIface ───────────────────────────────────────────────

// CurrencyRegistryClientIface abstracts on-chain CurrencyRegistry calls for CurrencyService.
type CurrencyRegistryClientIface interface {
	RegisterCurrency(ctx context.Context, symbol, countryName, tokenAddress, proposerCB string) (string, error)
	RemoveCurrency(ctx context.Context, symbol string) (string, error)
	GetAllCurrencies(ctx context.Context) ([]domain.CurrencyEntry, error)
}

// ── CurrencyService implementation ────────────────────────────────────────────

// CurrencyService implements CurrencyServiceIface.
type CurrencyService struct {
	client CurrencyRegistryClientIface
}

// NewCurrencyService creates a CurrencyService backed by an on-chain client.
func NewCurrencyService(client CurrencyRegistryClientIface) *CurrencyService {
	return &CurrencyService{client: client}
}

// RegisterCurrency validates the request and submits an on-chain registerCurrency transaction.
func (s *CurrencyService) RegisterCurrency(ctx context.Context, req CurrencyRegisterRequest) (*CurrencyRegisterResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("currency service: on-chain client not configured (CURRENCY_REGISTRY_CONTRACT_ADDRESS missing)")
	}
	if strings.TrimSpace(req.Symbol) == "" || strings.TrimSpace(req.CountryName) == "" ||
		strings.TrimSpace(req.TokenAddress) == "" || strings.TrimSpace(req.ProposerCB) == "" {
		return nil, fmt.Errorf("symbol, country_name, token_address, proposer_cb are required")
	}

	// Pre-check: detect duplicate symbol / token before submitting the tx so we can return a
	// meaningful domain error instead of the opaque "transaction reverted on-chain" message that
	// the EVM adapter emits when a Solidity custom error fires (same pattern as PairRegistry).
	if existing, err := s.client.GetAllCurrencies(ctx); err == nil {
		symbolNorm := strings.ToUpper(strings.TrimSpace(req.Symbol))
		addrNorm := strings.ToLower(strings.TrimSpace(req.TokenAddress))
		for _, e := range existing {
			if strings.ToUpper(e.Symbol) == symbolNorm {
				return nil, fmt.Errorf("%w: currency %q already registered", ErrCurrencyAlreadyExists, req.Symbol)
			}
			if strings.ToLower(e.TokenAddress) == addrNorm {
				return nil, fmt.Errorf("%w: token address %q already registered under a different symbol", ErrTokenAlreadyRegistered, req.TokenAddress)
			}
		}
	}

	txHash, err := s.client.RegisterCurrency(ctx, req.Symbol, req.CountryName, req.TokenAddress, req.ProposerCB)
	if err != nil {
		return nil, mapCurrencyOnChainError(err, req.Symbol, req.TokenAddress)
	}

	return &CurrencyRegisterResult{Symbol: req.Symbol, TxHash: txHash}, nil
}

// RemoveCurrency validates the request and submits an on-chain removeCurrency transaction.
func (s *CurrencyService) RemoveCurrency(ctx context.Context, req CurrencyRemoveRequest) (*CurrencyRemoveResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("currency service: on-chain client not configured (CURRENCY_REGISTRY_CONTRACT_ADDRESS missing)")
	}
	if strings.TrimSpace(req.Symbol) == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	txHash, err := s.client.RemoveCurrency(ctx, req.Symbol)
	if err != nil {
		return nil, mapCurrencyOnChainError(err, req.Symbol, "")
	}

	return &CurrencyRemoveResult{Symbol: req.Symbol, TxHash: txHash}, nil
}

// ListCurrencies reads all registered currencies from on-chain.
func (s *CurrencyService) ListCurrencies(ctx context.Context) ([]domain.CurrencyEntry, error) {
	if s.client == nil {
		return nil, fmt.Errorf("currency service: on-chain client not configured (CURRENCY_REGISTRY_CONTRACT_ADDRESS missing)")
	}
	entries, err := s.client.GetAllCurrencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("currency service list: %w", err)
	}
	if entries == nil {
		return []domain.CurrencyEntry{}, nil
	}
	return entries, nil
}

// mapCurrencyOnChainError converts on-chain revert messages to domain error sentinels.
func mapCurrencyOnChainError(err error, symbol, tokenAddress string) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "alreadyexists"):
		return fmt.Errorf("%w: currency %q already registered", ErrCurrencyAlreadyExists, symbol)
	case strings.Contains(msg, "tokenalreadyregistered"):
		return fmt.Errorf("%w: token address %q already registered under a different symbol", ErrTokenAlreadyRegistered, tokenAddress)
	case strings.Contains(msg, "notfound"):
		return fmt.Errorf("%w: currency %q not found", ErrCurrencyNotFound, symbol)
	case strings.Contains(msg, "unauthorized"):
		return fmt.Errorf("%w: caller is not the authorized issuer of this token", ErrCurrencyUnauthorized)
	default:
		return fmt.Errorf("currency registry on-chain error: %w", err)
	}
}
