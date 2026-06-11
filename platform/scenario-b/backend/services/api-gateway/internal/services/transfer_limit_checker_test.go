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

func TestChecker_NoLimitConfigured_Passes(t *testing.T) {
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: nil}, &mockVolumeRepo{})
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "500000")
	assert.NoError(t, err)
}

func TestChecker_WithinLimit_DeductsVolume(t *testing.T) {
	limit := &domain.TransferLimit{MaxAmount: "1000000000000000000000000"} // 1,000,000 tokens
	vol := &mockVolumeRepo{accumulated: "0"}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "500000")
	require.NoError(t, err)
	assert.NotEmpty(t, vol.deducted)
}

func TestChecker_LimitExceeded_ReturnsError(t *testing.T) {
	// max = 100 tokens, accumulated = 80 tokens, new = 30 tokens → total 110 > 100
	maxWei := "100000000000000000000"       // 100 * 10^18
	accWei := "80000000000000000000"        // 80 * 10^18
	limit := &domain.TransferLimit{MaxAmount: maxWei}
	vol := &mockVolumeRepo{accumulated: accWei}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "30")
	require.Error(t, err)
	var limitErr *ErrTransferLimitExceeded
	assert.ErrorAs(t, err, &limitErr)
	assert.Equal(t, "bank-a", limitErr.PayerBankID)
	assert.Equal(t, "BRL", limitErr.Currency)
}

func TestChecker_ExactlyAtLimit_Passes(t *testing.T) {
	// max = 100, accumulated = 70, new = 30 → total = 100 exactly (not exceeded)
	maxWei := "100000000000000000000"
	accWei := "70000000000000000000"
	limit := &domain.TransferLimit{MaxAmount: maxWei}
	vol := &mockVolumeRepo{accumulated: accWei}
	checker := NewTransferLimitChecker(&mockLimitRepo{limit: limit}, vol)

	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "30")
	assert.NoError(t, err)
}

func TestChecker_Restore_CallsVolumeRepo(t *testing.T) {
	vol := &mockVolumeRepo{}
	checker := NewTransferLimitChecker(&mockLimitRepo{}, vol)

	checker.Restore(context.Background(), "bank-a", "BRL", "500")
	assert.NotEmpty(t, vol.restored)
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
