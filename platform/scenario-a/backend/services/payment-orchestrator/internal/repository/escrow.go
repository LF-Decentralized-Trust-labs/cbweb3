// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.EscrowRepository = (*MemoryEscrowRepository)(nil)

type MemoryEscrowRepository struct {
	mu       sync.RWMutex
	deposits map[string]domain.DepositRecord
	escrows  map[string]domain.EscrowRecord
	redeems  map[string]domain.RedeemRecord
}

func NewMemoryEscrowRepository() *MemoryEscrowRepository {
	return &MemoryEscrowRepository{
		deposits: make(map[string]domain.DepositRecord),
		escrows:  make(map[string]domain.EscrowRecord),
		redeems:  make(map[string]domain.RedeemRecord),
	}
}

func (r *MemoryEscrowRepository) CreateDeposit(_ context.Context, record domain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.deposits[record.ID]; exists {
		return fmt.Errorf("deposit %s already exists", record.ID)
	}
	r.deposits[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) GetDeposit(_ context.Context, id string) (domain.DepositRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.deposits[id]
	return d, ok, nil
}

func (r *MemoryEscrowRepository) UpdateDeposit(_ context.Context, record domain.DepositRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.deposits[record.ID]; !exists {
		return fmt.Errorf("deposit %s not found", record.ID)
	}
	r.deposits[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) ListDeposits(_ context.Context, requesterID string) ([]domain.DepositRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.DepositRecord
	for _, d := range r.deposits {
		if requesterID == "" || d.RequesterID == requesterID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (r *MemoryEscrowRepository) CreateEscrow(_ context.Context, record domain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.escrows[record.ID]; exists {
		return fmt.Errorf("escrow %s already exists", record.ID)
	}
	r.escrows[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) GetEscrow(_ context.Context, id string) (domain.EscrowRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.escrows[id]
	return e, ok, nil
}

func (r *MemoryEscrowRepository) UpdateEscrow(_ context.Context, record domain.EscrowRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.escrows[record.ID]; !exists {
		return fmt.Errorf("escrow %s not found", record.ID)
	}
	r.escrows[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) ListEscrows(_ context.Context, requesterID string) ([]domain.EscrowRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.EscrowRecord
	for _, e := range r.escrows {
		if requesterID == "" || e.RequesterID == requesterID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *MemoryEscrowRepository) CreateRedeem(_ context.Context, record domain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.redeems[record.ID]; exists {
		return fmt.Errorf("redeem %s already exists", record.ID)
	}
	r.redeems[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) GetRedeem(_ context.Context, id string) (domain.RedeemRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rd, ok := r.redeems[id]
	return rd, ok, nil
}

func (r *MemoryEscrowRepository) UpdateRedeem(_ context.Context, record domain.RedeemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.redeems[record.ID]; !exists {
		return fmt.Errorf("redeem %s not found", record.ID)
	}
	r.redeems[record.ID] = record
	return nil
}

func (r *MemoryEscrowRepository) ListRedeems(_ context.Context, requesterID string) ([]domain.RedeemRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.RedeemRecord
	for _, rd := range r.redeems {
		if requesterID == "" || rd.RequesterID == requesterID {
			result = append(result, rd)
		}
	}
	return result, nil
}
