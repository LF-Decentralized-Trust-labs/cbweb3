// Package domain defines SwapQuote model for storing swap quotes with 15s TTL
// and server-side timestamp validation.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package domain

import "time"

// SwapQuote stores swap cotation with timestamp-based expiry (15s TTL) for
// server-side validation and replay attack protection.
type SwapQuote struct {
	QuoteID           string    `gorm:"primaryKey;column:quote_id;type:varchar(64)"`
	PoolPair          string    `gorm:"column:pool_pair;not null"`
	AmountOut         string    `gorm:"column:amount_out;not null"`
	AmountIn          string    `gorm:"column:amount_in;not null"`
	EffectiveRate     float64   `gorm:"column:effective_rate"`
	FeeBps            int       `gorm:"column:fee_bps;not null"`
	MaxSlippagePct    float64   `gorm:"column:max_slippage_pct;not null"`
	ReserveASnapshot  string    `gorm:"column:reserve_a_snapshot;not null"`
	ReserveBSnapshot  string    `gorm:"column:reserve_b_snapshot;not null"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	ValidUntil        time.Time `gorm:"column:valid_until;not null;index"`
}

// TableName overrides GORM's default table name.
func (SwapQuote) TableName() string {
	return "swap_quotes"
}

// IsExpired returns true if the quote has expired (server-side check).
func (q *SwapQuote) IsExpired() bool {
	return time.Now().After(q.ValidUntil)
}

// TimeRemainingSeconds returns seconds until expiry (used for countdown).
func (q *SwapQuote) TimeRemainingSeconds() float64 {
	remaining := time.Until(q.ValidUntil).Seconds()
	if remaining < 0 {
		return 0
	}
	return remaining
}
