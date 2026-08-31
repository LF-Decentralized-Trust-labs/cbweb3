// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock repos ---

type mockLimitRepo struct {
	limit *domain.TransferLimit
	err   error
}

func (m *mockLimitRepo) FindApplicableLimit(_ context.Context, _, _, _ string) (*domain.TransferLimit, error) {
	return m.limit, m.err
}

type mockVolumeRepo struct {
	accumulated string
	deductErr   error
	restoreErr  error
	deducted    string
	restored    string
}

func (m *mockVolumeRepo) Deduct(_ context.Context, _, _, amount string, _ time.Time) error {
	m.deducted = amount
	return m.deductErr
}
func (m *mockVolumeRepo) Restore(_ context.Context, _, _, amount string, _ time.Time) error {
	m.restored = amount
	return m.restoreErr
}
func (m *mockVolumeRepo) GetAccumulated(_ context.Context, _, _ string, _ time.Time) (string, error) {
	if m.accumulated == "" {
		return "0", nil
	}
	return m.accumulated, nil
}

// --- checker tests ---

// Amounts reaching the checker are in BASE UNITS (wei) — that is what the orchestrator, the
// bridge handler and the CB-internal pre-auth all pass. Only the configured limit is a human
// decimal string. Converting both is what made any configured limit reject every transfer.
const (
	oneToken      = "1000000000000000000"  // 1 * 10^18
	thirtyTokens  = "30000000000000000000" // 30 * 10^18
	seventyTokens = "70000000000000000000" // 70 * 10^18
	eightyTokens  = "80000000000000000000" // 80 * 10^18
)

func TestChecker_NoLimitConfigured_Passes(t *testing.T) {
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: nil}, &mockVolumeRepo{})
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "500000")
	assert.NoError(t, err)
}

func TestChecker_WithinLimit_DeductsVolume(t *testing.T) {
	limit := &domain.TransferLimit{MaxAmount: "1000000"} // 1,000,000 tokens (human-readable)
	vol := &mockVolumeRepo{accumulated: "0"}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	// 500,000 tokens in base units against a 1,000,000-token limit.
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "500000000000000000000000")
	require.NoError(t, err)
	assert.Equal(t, "500000000000000000000000", vol.deducted, "volume is recorded in the unit it arrived in")
}

func TestChecker_LimitExceeded_ReturnsError(t *testing.T) {
	// max = 100 tokens (human limit), accumulated = 80 tokens, new = 30 tokens → 110 > 100
	limit := &domain.TransferLimit{MaxAmount: "100"}
	vol := &mockVolumeRepo{accumulated: eightyTokens}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", thirtyTokens)
	require.Error(t, err)
	var limitErr *ErrTransferLimitExceeded
	assert.ErrorAs(t, err, &limitErr)
	assert.Equal(t, "bank-a", limitErr.PayerBankID)
	assert.Equal(t, "BRL", limitErr.Currency)
}

func TestChecker_ExactlyAtLimit_Passes(t *testing.T) {
	// max = 100 tokens (human limit), accumulated = 70 tokens, new = 30 tokens → exactly 100
	limit := &domain.TransferLimit{MaxAmount: "100"}
	vol := &mockVolumeRepo{accumulated: seventyTokens}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", thirtyTokens)
	assert.NoError(t, err)
}

// TestChecker_BaseUnitAmountAgainstHumanLimit is the regression guard for the unit bug: a
// single-token payment must not breach a 100-token limit. While the amount was also multiplied
// by 10^18, this compared 10^36 against 10^20 and rejected every transfer as over the limit.
func TestChecker_BaseUnitAmountAgainstHumanLimit(t *testing.T) {
	limit := &domain.TransferLimit{MaxAmount: "100"}
	vol := &mockVolumeRepo{accumulated: "0"}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", oneToken)
	require.NoError(t, err, "1 token must fit under a 100-token daily limit")
	assert.Equal(t, oneToken, vol.deducted)
}

// Deduct and Restore must move the same number, or a failed swap leaves the quota drifting.
func TestChecker_RestoreMirrorsDeductedUnit(t *testing.T) {
	limit := &domain.TransferLimit{MaxAmount: "100"}
	vol := &mockVolumeRepo{accumulated: "0"}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	require.NoError(t, checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", thirtyTokens))
	checker.Restore(context.Background(), "bank-a", "BRL", thirtyTokens)
	assert.Equal(t, vol.deducted, vol.restored, "restoring a different unit than deducted drifts the quota")
}

func TestChecker_Restore_CallsVolumeRepo(t *testing.T) {
	vol := &mockVolumeRepo{}
	checker := NewTransferLimitChecker(&mockLimitRepo{}, vol)

	checker.Restore(context.Background(), "bank-a", "BRL", "500")
	assert.Equal(t, "500", vol.restored, "the amount is restored in the unit it arrived in")
}

func TestChecker_UnknownSpokePrefix_Passes(t *testing.T) {
	// bank with no recognizable suffix → no CB → skip limit check
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: nil}, &mockVolumeRepo{})
	err := checker.CheckAndDeduct(context.Background(), "unknown-bank", "BRL", "999999999")
	assert.NoError(t, err)
}

// --- humanToWei unit tests ---

func TestHumanToWei_WholeNumber(t *testing.T) {
	result, err := humanToWei("1")
	require.NoError(t, err)
	expected := "1000000000000000000"
	assert.Equal(t, expected, result.String())
}

func TestHumanToWei_WithDecimals(t *testing.T) {
	result, err := humanToWei("1.5")
	require.NoError(t, err)
	expected := "1500000000000000000"
	assert.Equal(t, expected, result.String())
}

func TestHumanToWei_LargeAmount(t *testing.T) {
	result, err := humanToWei("1000000")
	require.NoError(t, err)
	expected := "1000000000000000000000000"
	assert.Equal(t, expected, result.String())
}

func TestHumanToWei_InvalidInput(t *testing.T) {
	_, err := humanToWei("abc")
	assert.Error(t, err)
}

// --- centralBankIDForPayer tests ---

func TestCentralBankIDForPayer(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"bank-a", "central-bank-a"},
		{"bank-b", "central-bank-b"},
		{"central-bank-a", "central-bank-a"},
		{"central-bank-b", "central-bank-b"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.expected, centralBankIDForPayer(tc.input), "input: %s", tc.input)
	}
}
