// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/workers"
)

// A pending on-chain operation must survive a restart: it is what keeps the server from
// submitting a second transaction for the agreement, and what the reconciler resolves.
func TestGormFXAgreementRepository_PendingOperationRoundTrip(t *testing.T) {
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(newTestDB(t))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	store, ok := repo.(workers.PendingFXStore)
	if !ok {
		t.Fatal("the GORM repository does not implement workers.PendingFXStore; the reconciler would not start")
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	mk := func(id string, st domain.FXState, p domain.FXPending) *domain.FXAgreementRecord {
		return &domain.FXAgreementRecord{
			TradeID: id, Originator: "bank-a", CounterpartyB: "bank-b",
			OriginAmount: "100", CounterAmount: "120", OriginCurrency: "USD", CounterCurrency: "BRL",
			Rate: "1.2", ExpiryDate: uint64(now.Add(time.Hour).Unix()), State: st, Pending: p,
			CreatedAt: now, UpdatedAt: now,
		}
	}

	proposeP := domain.FXPending{Op: domain.FXPendingPropose, TxID: "ptx-1", Since: now.Add(-time.Minute)}
	if err := repo.CreateAgreement(ctx, mk("P1", domain.FXStateInvalid, proposeP)); err != nil {
		t.Fatalf("create pending: %v", err)
	}
	if err := repo.CreateAgreement(ctx, mk("N1", domain.FXStateProposed, domain.FXPending{})); err != nil {
		t.Fatalf("create plain: %v", err)
	}
	accepting := mk("A1", domain.FXStateProposed, domain.FXPending{})
	if err := repo.CreateAgreement(ctx, accepting); err != nil {
		t.Fatalf("create: %v", err)
	}
	// An unconfirmed accept recorded on an existing agreement, with no transaction id known.
	accepting.Pending = domain.FXPending{Op: domain.FXPendingAccept, Since: now}
	if err := repo.UpdateAgreement(ctx, accepting); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetAgreement(ctx, "P1")
	if err != nil || got == nil {
		t.Fatalf("get: %+v %v", got, err)
	}
	if got.Pending.Op != domain.FXPendingPropose || got.Pending.TxID != "ptx-1" || !got.Pending.Since.Equal(proposeP.Since) {
		t.Errorf("pending not persisted: %+v", got.Pending)
	}

	pending, err := store.ListPendingAgreements(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 2 || pending[0].TradeID != "P1" || pending[1].TradeID != "A1" {
		ids := []string{}
		for _, r := range pending {
			ids = append(ids, r.TradeID)
		}
		t.Fatalf("pending = %v, want [P1 A1] (oldest first, plain agreements excluded)", ids)
	}

	// Clearing removes it from the list.
	accepting.Pending = domain.FXPending{}
	accepting.State = domain.FXStateAccepted
	if err := repo.UpdateAgreement(ctx, accepting); err != nil {
		t.Fatalf("clear: %v", err)
	}
	pending, _ = store.ListPendingAgreements(ctx, 10)
	if len(pending) != 1 || pending[0].TradeID != "P1" {
		t.Fatalf("after clearing A1, pending = %d records, want only P1", len(pending))
	}
	got, _ = repo.GetAgreement(ctx, "A1")
	if got.Pending.IsSet() || got.State != domain.FXStateAccepted {
		t.Errorf("A1 after clearing: state=%s pending=%+v", got.State, got.Pending)
	}
}
