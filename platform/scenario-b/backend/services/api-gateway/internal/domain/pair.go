package domain

import "time"

// PairStatusProposed and PairStatusActive are the two lifecycle states
// of a currency pair in PairRegistry (D9 — 005-cooperative-liquidity).
const (
	PairStatusProposed = "PROPOSED"
	PairStatusActive   = "ACTIVE"
)

// PairProposal represents a row in the pair_proposals table.
// It mirrors the on-chain PairEntry state and is kept in sync by the
// PairRouter goroutine that subscribes to PairRegistered events (D10/D11).
type PairProposal struct {
	PairID        string     `gorm:"primaryKey;column:pair_id"`
	ProposerCB    string     `gorm:"not null;column:proposer_cb"`
	ConfirmerCB   string     `gorm:"column:confirmer_cb"`
	TokenAAddress string     `gorm:"not null;column:token_a_address"`
	TokenBAddress string     `gorm:"not null;column:token_b_address"`
	AMMAddress    string     `gorm:"not null;column:amm_address"`
	Status        string     `gorm:"not null;default:PROPOSED;check:status IN ('PROPOSED','ACTIVE');column:status"`
	ProposedAt    time.Time  `gorm:"not null;autoCreateTime;column:proposed_at"`
	ConfirmedAt   *time.Time `gorm:"column:confirmed_at"`
}

// TableName sets the PostgreSQL table name for GORM AutoMigrate.
func (PairProposal) TableName() string { return "pair_proposals" }

// PairEntry is the in-memory routing entry used by PairRouter.
// It is derived from PairProposal rows and from on-chain getAllActivePairs (D10).
type PairEntry struct {
	PairID     string
	AMMAddress string
	TokenA     string
	TokenB     string
}
