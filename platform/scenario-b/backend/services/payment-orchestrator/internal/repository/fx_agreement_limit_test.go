// SPDX-License-Identifier: Apache-2.0

// Bound ListAgreements (finding R2-M-14).
//
// The query was `q.Order("created_at DESC").Find(&models)` with no LIMIT: every row matching
// the filter was loaded into memory and serialised, and the filter is optional — an empty one
// matches the whole table. The gRPC request carries no page size, so no caller could bound it
// even if it wanted to.
//
// This is a real repository test against in-memory SQLite rather than a check on a helper,
// because the claim worth pinning is "the database returns at most N rows", not "a function
// returns N".
package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/repository"
)

// newLimitTestDB returns an in-memory SQLite handle. Uniquely named so it does not collide
// with the package's existing test helpers where those exist.
func newLimitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:fxlimit?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// newLimitRepo builds the repository over that handle (the FromDB constructor AutoMigrates).
func newLimitRepo(t *testing.T, db *gorm.DB) ports.FXAgreementRepository {
	t.Helper()
	repo, err := repository.NewGormFXAgreementRepositoryFromDB(db)
	if err != nil {
		t.Fatalf("repository: %v", err)
	}
	return repo
}

// seedAgreements inserts n agreements with distinct trade ids and increasing created_at.
func seedAgreements(t *testing.T, repo ports.FXAgreementRepository, n int) {
	t.Helper()
	base := time.Now().Add(-time.Duration(n) * time.Minute)
	for i := 0; i < n; i++ {
		rec := &domain.FXAgreementRecord{
			TradeID:       fmt.Sprintf("LIMIT-TRADE-%04d", i),
			Originator:    "bank-a",
			CounterpartyB: "bank-b",
			State:         domain.FXStateProposed,
			CreatedAt:     base.Add(time.Duration(i) * time.Minute),
		}
		if err := repo.CreateAgreement(context.Background(), rec); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
}

// An unfiltered list must not return the whole table.
func TestListAgreements_IsBounded(t *testing.T) {
	db := newLimitTestDB(t)
	repo := newLimitRepo(t, db)

	over := repository.MaxFXAgreementPageSize + 10
	seedAgreements(t, repo, over)

	got, err := repo.ListAgreements(context.Background(), ports.FXAgreementFilter{})
	if err != nil {
		t.Fatalf("ListAgreements: %v", err)
	}
	if len(got) > repository.MaxFXAgreementPageSize {
		t.Errorf("returned %d rows from a table of %d: the query is unbounded (cap is %d)",
			len(got), over, repository.MaxFXAgreementPageSize)
	}
}

// A caller may ask for fewer, and must not be able to ask for more.
func TestListAgreements_RespectsAndCapsTheRequestedLimit(t *testing.T) {
	db := newLimitTestDB(t)
	repo := newLimitRepo(t, db)
	seedAgreements(t, repo, repository.MaxFXAgreementPageSize+10)

	small, err := repo.ListAgreements(context.Background(), ports.FXAgreementFilter{Limit: 5})
	if err != nil {
		t.Fatalf("ListAgreements(5): %v", err)
	}
	if len(small) != 5 {
		t.Errorf("Limit=5 returned %d rows, want 5", len(small))
	}

	huge, err := repo.ListAgreements(context.Background(), ports.FXAgreementFilter{Limit: 1_000_000})
	if err != nil {
		t.Fatalf("ListAgreements(1e6): %v", err)
	}
	if len(huge) > repository.MaxFXAgreementPageSize {
		t.Errorf("Limit=1000000 returned %d rows, above the %d cap", len(huge), repository.MaxFXAgreementPageSize)
	}
}

// The newest agreements are the ones a bounded page must carry, since the query orders by
// created_at DESC — truncating the wrong end would silently hide recent activity.
func TestListAgreements_BoundedPageKeepsTheNewest(t *testing.T) {
	db := newLimitTestDB(t)
	repo := newLimitRepo(t, db)
	total := repository.MaxFXAgreementPageSize + 10
	seedAgreements(t, repo, total)

	got, err := repo.ListAgreements(context.Background(), ports.FXAgreementFilter{Limit: 3})
	if err != nil {
		t.Fatalf("ListAgreements: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	newest := fmt.Sprintf("LIMIT-TRADE-%04d", total-1)
	if got[0].TradeID != newest {
		t.Errorf("first row is %s, want the newest %s", got[0].TradeID, newest)
	}
}
