// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// DepositModel is the GORM model for the deposits table.
type DepositModel struct {
	ID                       string    `gorm:"primaryKey;column:id"`
	RequesterID              string    `gorm:"not null;column:requester_id;index:idx_deposits_requester_id"`
	RequesterBesuAddress     string    `gorm:"not null;column:requester_besu_address"`
	RequesterPaladinIdentity string    `gorm:"not null;column:requester_paladin_identity"`
	Amount                   string    `gorm:"not null;column:amount"`
	Status                   string    `gorm:"not null;column:status"`
	MintTxHash               string    `gorm:"column:mint_tx_hash"`
	RejectionReason          string    `gorm:"column:rejection_reason"`
	CreatedAt                time.Time `gorm:"autoCreateTime"`
}

func (DepositModel) TableName() string { return "deposits" }

// EscrowModel is the GORM model for the escrows table.
type EscrowModel struct {
	ID                       string    `gorm:"primaryKey;column:id"`
	RequesterID              string    `gorm:"not null;column:requester_id;index:idx_escrows_requester_id"`
	RequesterBesuAddress     string    `gorm:"not null;column:requester_besu_address"`
	RequesterPaladinIdentity string    `gorm:"not null;column:requester_paladin_identity"`
	Amount                   string    `gorm:"not null;column:amount"`
	Status                   string    `gorm:"not null;column:status"`
	BurnTxHash               string    `gorm:"column:burn_tx_hash"`
	MintTxHash               string    `gorm:"column:mint_tx_hash"`
	RejectionReason          string    `gorm:"column:rejection_reason"`
	CreatedAt                time.Time `gorm:"autoCreateTime"`
}

func (EscrowModel) TableName() string { return "escrows" }

// RedeemModel is the GORM model for the redeems table.
type RedeemModel struct {
	ID                       string    `gorm:"primaryKey;column:id"`
	RequesterID              string    `gorm:"not null;column:requester_id;index:idx_redeems_requester_id"`
	RequesterBesuAddress     string    `gorm:"not null;column:requester_besu_address"`
	RequesterPaladinIdentity string    `gorm:"not null;column:requester_paladin_identity"`
	Amount                   string    `gorm:"not null;column:amount"`
	Status                   string    `gorm:"not null;column:status"`
	ZetoTransferTxHash       string    `gorm:"column:zeto_transfer_tx_hash"`
	FiatMintTxHash           string    `gorm:"column:fiat_mint_tx_hash"`
	RejectionReason          string    `gorm:"column:rejection_reason"`
	CreatedAt                time.Time `gorm:"autoCreateTime"`
}

func (RedeemModel) TableName() string { return "redeems" }

// --- Model ↔ Domain converters ---

func depositToModel(r domain.DepositRecord) DepositModel {
	return DepositModel{
		ID:                       r.ID,
		RequesterID:              r.RequesterID,
		RequesterBesuAddress:     r.RequesterBesuAddress,
		RequesterPaladinIdentity: r.RequesterPaladinIdentity,
		Amount:                   r.Amount,
		Status:                   string(r.Status),
		MintTxHash:               r.MintTxHash,
		RejectionReason:          r.RejectionReason,
		CreatedAt:                r.CreatedAt,
	}
}

func depositFromModel(m DepositModel) domain.DepositRecord {
	return domain.DepositRecord{
		ID:                       m.ID,
		RequesterID:              m.RequesterID,
		RequesterBesuAddress:     m.RequesterBesuAddress,
		RequesterPaladinIdentity: m.RequesterPaladinIdentity,
		Amount:                   m.Amount,
		Status:                   domain.DepositStatus(m.Status),
		MintTxHash:               m.MintTxHash,
		RejectionReason:          m.RejectionReason,
		CreatedAt:                m.CreatedAt,
	}
}

func escrowToModel(r domain.EscrowRecord) EscrowModel {
	return EscrowModel{
		ID:                       r.ID,
		RequesterID:              r.RequesterID,
		RequesterBesuAddress:     r.RequesterBesuAddress,
		RequesterPaladinIdentity: r.RequesterPaladinIdentity,
		Amount:                   r.Amount,
		Status:                   string(r.Status),
		BurnTxHash:               r.BurnTxHash,
		MintTxHash:               r.MintTxHash,
		RejectionReason:          r.RejectionReason,
		CreatedAt:                r.CreatedAt,
	}
}

func escrowFromModel(m EscrowModel) domain.EscrowRecord {
	return domain.EscrowRecord{
		ID:                       m.ID,
		RequesterID:              m.RequesterID,
		RequesterBesuAddress:     m.RequesterBesuAddress,
		RequesterPaladinIdentity: m.RequesterPaladinIdentity,
		Amount:                   m.Amount,
		Status:                   domain.EscrowStatus(m.Status),
		BurnTxHash:               m.BurnTxHash,
		MintTxHash:               m.MintTxHash,
		RejectionReason:          m.RejectionReason,
		CreatedAt:                m.CreatedAt,
	}
}

func redeemToModel(r domain.RedeemRecord) RedeemModel {
	return RedeemModel{
		ID:                       r.ID,
		RequesterID:              r.RequesterID,
		RequesterBesuAddress:     r.RequesterBesuAddress,
		RequesterPaladinIdentity: r.RequesterPaladinIdentity,
		Amount:                   r.Amount,
		Status:                   string(r.Status),
		ZetoTransferTxHash:       r.ZetoTransferTxHash,
		FiatMintTxHash:           r.FiatMintTxHash,
		RejectionReason:          r.RejectionReason,
		CreatedAt:                r.CreatedAt,
	}
}

func redeemFromModel(m RedeemModel) domain.RedeemRecord {
	return domain.RedeemRecord{
		ID:                       m.ID,
		RequesterID:              m.RequesterID,
		RequesterBesuAddress:     m.RequesterBesuAddress,
		RequesterPaladinIdentity: m.RequesterPaladinIdentity,
		Amount:                   m.Amount,
		Status:                   domain.RedeemStatus(m.Status),
		ZetoTransferTxHash:       m.ZetoTransferTxHash,
		FiatMintTxHash:           m.FiatMintTxHash,
		RejectionReason:          m.RejectionReason,
		CreatedAt:                m.CreatedAt,
	}
}
