package repository

import (
	"context"
	"testing"
	"time"
)

// ── CreateTransferLimit / ListTransferLimits ──────────────────────────────────

func TestMemoryCreateAndListTransferLimits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	limit := TransferLimit{
		LimitID:       "lim-1",
		CentralBankID: "cb-a",
		ParticipantID: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "500000",
		IsActive:      true,
	}
	if err := repo.CreateTransferLimit(ctx, limit); err != nil {
		t.Fatalf("create: %v", err)
	}

	limits, err := repo.ListTransferLimits(ctx, "cb-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(limits) != 1 {
		t.Fatalf("expected 1 limit, got %d", len(limits))
	}
	if limits[0].LimitID != "lim-1" {
		t.Errorf("unexpected limit_id: %q", limits[0].LimitID)
	}
}

func TestMemoryListTransferLimits_FiltersByCentralBank(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "l1", CentralBankID: "cb-a", IsActive: true, MaxAmount: "1"})
	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "l2", CentralBankID: "cb-b", IsActive: true, MaxAmount: "1"})

	got, _ := repo.ListTransferLimits(ctx, "cb-a")
	if len(got) != 1 || got[0].LimitID != "l1" {
		t.Errorf("expected only cb-a limit, got %+v", got)
	}
}

// ── DeleteTransferLimit ───────────────────────────────────────────────────────

func TestMemoryDeleteTransferLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "lim-del", CentralBankID: "cb-a", IsActive: true, MaxAmount: "1"})
	if err := repo.DeleteTransferLimit(ctx, "lim-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	limits, _ := repo.ListTransferLimits(ctx, "cb-a")
	for _, l := range limits {
		if l.LimitID == "lim-del" {
			t.Errorf("deleted limit still visible in list")
		}
	}
}

// ── FindApplicableLimit — priority cascade ────────────────────────────────────

func TestMemoryFindApplicableLimit_ExactMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	// Exact match (pid+cur) must win over wildcard.
	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "exact", CentralBankID: "cb-a", ParticipantID: "bank-a", Currency: "BRL", IsActive: true, MaxAmount: "100"})
	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "wildcard", CentralBankID: "cb-a", IsActive: true, MaxAmount: "999"})

	got, err := repo.FindApplicableLimit(ctx, "cb-a", "bank-a", "BRL")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got == nil {
		t.Fatal("expected a limit, got nil")
	}
	if got.LimitID != "exact" {
		t.Errorf("expected exact-match limit, got %q (max=%q)", got.LimitID, got.MaxAmount)
	}
}

func TestMemoryFindApplicableLimit_WildcardMatchesAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "wild", CentralBankID: "cb-a", ParticipantID: "", Currency: "", IsActive: true, MaxAmount: "50000"})

	got, err := repo.FindApplicableLimit(ctx, "cb-a", "any-bank", "EUR")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got == nil {
		t.Fatal("expected wildcard limit to match, got nil")
	}
	if got.LimitID != "wild" {
		t.Errorf("expected wildcard limit, got %q", got.LimitID)
	}
}

func TestMemoryFindApplicableLimit_NilWhenNoLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	got, err := repo.FindApplicableLimit(ctx, "cb-a", "bank-x", "USD")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when no limit configured, got %+v", got)
	}
}

func TestMemoryFindApplicableLimit_InactiveNotReturned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "inactive", CentralBankID: "cb-a", IsActive: false, MaxAmount: "100"})

	got, err := repo.FindApplicableLimit(ctx, "cb-a", "bank-a", "BRL")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for inactive limit, got %+v", got)
	}
}

func TestMemoryFindApplicableLimit_ParticipantOnlyFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	// pid-only limit: any currency, specific participant.
	_ = repo.CreateTransferLimit(ctx, TransferLimit{LimitID: "pid-only", CentralBankID: "cb-a", ParticipantID: "bank-a", Currency: "", IsActive: true, MaxAmount: "200"})

	got, err := repo.FindApplicableLimit(ctx, "cb-a", "bank-a", "EUR")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got == nil {
		t.Fatal("expected pid-only limit to match, got nil")
	}
	if got.LimitID != "pid-only" {
		t.Errorf("expected pid-only limit, got %q", got.LimitID)
	}
}

// ── Volume deduction / restoration ───────────────────────────────────────────

func TestMemoryGetAccumulatedVolume_ZeroWhenEmpty(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()

	got, err := repo.GetAccumulatedVolume(context.Background(), "bank-a", "BRL", time.Now().UTC())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "0" {
		t.Errorf("expected '0', got %q", got)
	}
}

func TestMemoryDeductTransferVolume_Accumulates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()
	day := time.Now().UTC()

	if err := repo.DeductTransferVolume(ctx, "bank-a", "BRL", "1000", day); err != nil {
		t.Fatalf("first deduct: %v", err)
	}
	if err := repo.DeductTransferVolume(ctx, "bank-a", "BRL", "500", day); err != nil {
		t.Fatalf("second deduct: %v", err)
	}

	got, err := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", day)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "1500" {
		t.Errorf("expected '1500', got %q", got)
	}
}

func TestMemoryRestoreTransferVolume_ReducesBalance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()
	day := time.Now().UTC()

	_ = repo.DeductTransferVolume(ctx, "bank-a", "BRL", "2000", day)
	if err := repo.RestoreTransferVolume(ctx, "bank-a", "BRL", "500", day); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, _ := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", day)
	if got != "1500" {
		t.Errorf("expected '1500' after restore, got %q", got)
	}
}

func TestMemoryRestoreTransferVolume_DoesNotGoBelowZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()
	day := time.Now().UTC()

	_ = repo.DeductTransferVolume(ctx, "bank-a", "BRL", "100", day)
	if err := repo.RestoreTransferVolume(ctx, "bank-a", "BRL", "999999", day); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, _ := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", day)
	if got != "0" {
		t.Errorf("expected '0' (floor at zero), got %q", got)
	}
}

func TestMemoryDeductTransferVolume_IsolatedByDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := NewMemoryRepository()

	today := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	_ = repo.DeductTransferVolume(ctx, "bank-a", "BRL", "1000", today)
	_ = repo.DeductTransferVolume(ctx, "bank-a", "BRL", "2000", yesterday)

	todayVol, _ := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", today)
	yesterdayVol, _ := repo.GetAccumulatedVolume(ctx, "bank-a", "BRL", yesterday)

	if todayVol != "1000" {
		t.Errorf("today: expected '1000', got %q", todayVol)
	}
	if yesterdayVol != "2000" {
		t.Errorf("yesterday: expected '2000', got %q", yesterdayVol)
	}
}
