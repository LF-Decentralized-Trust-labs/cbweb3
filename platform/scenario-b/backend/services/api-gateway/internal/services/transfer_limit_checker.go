// Package services provides TransferLimitChecker for enforcing CB transfer limits (R1-10.1).
package services

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// tokenDecimals is the fixed decimal precision for all tokens in this system (ERC-20 default).
const tokenDecimals = 18

// TransferLimitRepo is the repository interface consumed by TransferLimitChecker.
type TransferLimitRepo interface {
	FindApplicableLimit(ctx context.Context, centralBankID, participantID, currency string) (*domain.TransferLimit, error)
}

// TransferVolumeRepo is the repository interface consumed by TransferLimitChecker.
type TransferVolumeRepo interface {
	Deduct(ctx context.Context, participantID, currency, amount string, windowDate time.Time) error
	Restore(ctx context.Context, participantID, currency, amount string, windowDate time.Time) error
	GetAccumulated(ctx context.Context, participantID, currency string, windowDate time.Time) (string, error)
}

// TransferLimitCheckerIface is the interface consumed by the swap orchestrator and bridge handler.
type TransferLimitCheckerIface interface {
	// CheckAndDeduct validates the transfer against the applicable daily limit and records volume.
	// amountHuman is a human-readable decimal string (e.g. "1000.50"); converted to wei internally.
	CheckAndDeduct(ctx context.Context, payerBankID, currency, amountHuman string) error
	// Restore returns a previously deducted amount to the daily quota on synchronous failure.
	Restore(ctx context.Context, payerBankID, currency, amountHuman string)
}

// ErrTransferLimitExceeded is returned when a transfer would breach the configured daily limit.
type ErrTransferLimitExceeded struct {
	PayerBankID string
	Currency    string
	MaxAmount   string
}

func (e *ErrTransferLimitExceeded) Error() string {
	return fmt.Sprintf("daily transfer limit exceeded for %s/%s (limit: %s wei)", e.PayerBankID, e.Currency, e.MaxAmount)
}

// TransferLimitChecker implements TransferLimitCheckerIface.
type TransferLimitChecker struct {
	limitRepo  TransferLimitRepo
	volumeRepo TransferVolumeRepo
}

// NewTransferLimitChecker creates a TransferLimitChecker.
func NewTransferLimitChecker(limitRepo TransferLimitRepo, volumeRepo TransferVolumeRepo) *TransferLimitChecker {
	return &TransferLimitChecker{limitRepo: limitRepo, volumeRepo: volumeRepo}
}

// CheckAndDeduct validates and records the transfer volume against the daily limit.
func (c *TransferLimitChecker) CheckAndDeduct(ctx context.Context, payerBankID, currency, amountHuman string) error {
	cbID := centralBankIDForPayer(payerBankID)
	if cbID == "" {
		return nil
	}

	amountWei, err := humanToWei(amountHuman)
	if err != nil {
		return fmt.Errorf("invalid transfer amount %q: %w", amountHuman, err)
	}

	limit, err := c.limitRepo.FindApplicableLimit(ctx, cbID, payerBankID, currency)
	if err != nil {
		return fmt.Errorf("transfer limit lookup failed: %w", err)
	}
	if limit == nil {
		return nil
	}

	maxWei, err := humanToWei(limit.MaxAmount)
	if err != nil {
		return fmt.Errorf("malformed limit max_amount %q: %w", limit.MaxAmount, err)
	}

	today := utcDay(time.Now().UTC())
	accumulatedStr, err := c.volumeRepo.GetAccumulated(ctx, payerBankID, currency, today)
	if err != nil {
		return fmt.Errorf("transfer volume lookup failed: %w", err)
	}
	accumulatedWei, err := weiFromString(accumulatedStr)
	if err != nil {
		return fmt.Errorf("malformed accumulated_amount %q: %w", accumulatedStr, err)
	}

	if new(big.Int).Add(accumulatedWei, amountWei).Cmp(maxWei) > 0 {
		return &ErrTransferLimitExceeded{
			PayerBankID: payerBankID,
			Currency:    currency,
			MaxAmount:   limit.MaxAmount,
		}
	}

	if err := c.volumeRepo.Deduct(ctx, payerBankID, currency, amountWei.String(), today); err != nil {
		return fmt.Errorf("transfer volume deduct failed: %w", err)
	}
	return nil
}

// Restore returns a previously deducted volume on synchronous failure.
// Errors are swallowed — this is a best-effort compensation call.
func (c *TransferLimitChecker) Restore(ctx context.Context, payerBankID, currency, amountHuman string) {
	amountWei, err := humanToWei(amountHuman)
	if err != nil {
		return
	}
	today := utcDay(time.Now().UTC())
	_ = c.volumeRepo.Restore(ctx, payerBankID, currency, amountWei.String(), today)
}

// centralBankIDForPayer derives the governing CB ID from the payer bank code suffix.
// "bank-a" / "central-bank-a" → "central-bank-a"; "bank-b" → "central-bank-b".
func centralBankIDForPayer(payerBankID string) string {
	lower := strings.ToLower(strings.TrimSpace(payerBankID))
	if strings.HasSuffix(lower, "-a") {
		return "central-bank-a"
	}
	if strings.HasSuffix(lower, "-b") {
		return "central-bank-b"
	}
	return ""
}

// humanToWei converts a human-readable decimal string (e.g. "1000.50") to wei (×10^18).
func humanToWei(human string) (*big.Int, error) {
	human = strings.TrimSpace(human)
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)

	parts := strings.SplitN(human, ".", 2)
	intPart, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer part %q", parts[0])
	}
	result := new(big.Int).Mul(intPart, multiplier)

	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > tokenDecimals {
			frac = frac[:tokenDecimals]
		} else {
			frac = frac + strings.Repeat("0", tokenDecimals-len(frac))
		}
		fracInt, ok := new(big.Int).SetString(frac, 10)
		if !ok {
			return nil, fmt.Errorf("invalid fractional part %q", frac)
		}
		result.Add(result, fracInt)
	}
	return result, nil
}

// weiFromString parses a base-10 wei string into *big.Int.
func weiFromString(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "0"
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid wei string %q", s)
	}
	return n, nil
}

// utcDay truncates t to UTC midnight, giving the daily window key.
func utcDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
