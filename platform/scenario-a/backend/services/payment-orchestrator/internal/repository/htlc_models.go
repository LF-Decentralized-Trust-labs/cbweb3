package repository

import (
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// HTLCModel is the GORM model for the htlcs table.
type HTLCModel struct {
	ContractID  string    `gorm:"primaryKey;column:contract_id"`
	AgreementID string    `gorm:"column:agreement_id;index:idx_htlcs_agreement_id"`
	Sender      string    `gorm:"not null;column:sender;index:idx_htlcs_sender"`
	Receiver    string    `gorm:"not null;column:receiver;index:idx_htlcs_receiver"`
	Amount      string    `gorm:"not null;column:amount"`
	HashLock    string    `gorm:"not null;column:hash_lock;uniqueIndex:idx_htlcs_hash_lock"`
	TimeLock    uint64    `gorm:"not null;column:time_lock"`
	Secret             string `gorm:"column:secret"`
	ZetoLockRef        string `gorm:"column:zeto_lock_ref"`
	State              string `gorm:"not null;column:state;index:idx_htlcs_state"`
	HTLCTxHash         string `gorm:"column:htlc_tx_hash"`
	ZetoTxHash         string `gorm:"column:zeto_tx_hash"`
	CounterpartyLocked bool   `gorm:"not null;column:counterparty_locked;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (HTLCModel) TableName() string { return "htlcs" }

// --- Model ↔ Domain converters ---

func htlcToModel(r *domain.HTLCRecord) HTLCModel {
	return HTLCModel{
		ContractID:         r.ContractID,
		AgreementID:        r.AgreementID,
		Sender:             r.Sender,
		Receiver:           r.Receiver,
		Amount:             r.Amount,
		HashLock:           r.HashLock,
		TimeLock:           r.TimeLock,
		Secret:             r.Secret,
		ZetoLockRef:        r.ZetoLockRef,
		State:              string(r.State),
		HTLCTxHash:         r.HTLCTxHash,
		ZetoTxHash:         r.ZetoTxHash,
		CounterpartyLocked: r.CounterpartyLocked,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func htlcFromModel(m HTLCModel) *domain.HTLCRecord {
	return &domain.HTLCRecord{
		ContractID:         m.ContractID,
		AgreementID:        m.AgreementID,
		Sender:             m.Sender,
		Receiver:           m.Receiver,
		Amount:             m.Amount,
		HashLock:           m.HashLock,
		TimeLock:           m.TimeLock,
		Secret:             m.Secret,
		ZetoLockRef:        m.ZetoLockRef,
		State:              domain.HTLCState(m.State),
		HTLCTxHash:         m.HTLCTxHash,
		ZetoTxHash:         m.ZetoTxHash,
		CounterpartyLocked: m.CounterpartyLocked,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}
