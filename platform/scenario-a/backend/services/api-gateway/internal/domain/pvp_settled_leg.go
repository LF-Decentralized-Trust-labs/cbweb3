// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// PvPSettledLeg is a settled inter-bank PvP settlement leg reported to the
// Central Bank by the settling orchestrator. It is the durable source of the
// credit side of a receiving bank's statement: a receiving bank's own
// orchestrator holds no record of an incoming leg (the counterparty locked it
// elsewhere and the amount is private), so the CB persists the reported fact and
// serves each bank the legs on which it is the receiver.
//
// ContractID is the primary key so re-reports (settle retries, relay redelivery)
// upsert idempotently rather than duplicating a movement.
type PvPSettledLeg struct {
	ContractID     string    `gorm:"primaryKey;column:contract_id;type:varchar(80)"`
	TradeID        string    `gorm:"column:trade_id;index"`
	Sender         string    `gorm:"column:sender"`
	Receiver       string    `gorm:"column:receiver"`
	ReceiverBankID string    `gorm:"column:receiver_bank_id;index"` // parsed from Receiver, for scoped queries
	Amount         string    `gorm:"column:amount"`
	SettledAt      time.Time `gorm:"column:settled_at"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName pins the table name for GORM.
func (PvPSettledLeg) TableName() string { return "pvp_settled_legs" }
