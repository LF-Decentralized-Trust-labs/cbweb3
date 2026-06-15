// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOversightDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.DisclosureRequest{}, &domain.DisclosureSignature{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestOpenDisclosure_Success(t *testing.T) {
	svc := NewOversightService(newOversightDB(t))
	req, err := svc.OpenDisclosure(context.Background(), "tx-ref-1", "bank-a", "AML_INVESTIGATION")
	if err != nil {
		t.Fatalf("OpenDisclosure: %v", err)
	}
	if req.State != domain.DisclosurePending {
		t.Errorf("state = %q, want PENDING", req.State)
	}
	if req.QuorumRequired != 2 || req.QuorumReached != 0 {
		t.Errorf("quorum required/reached = %d/%d, want 2/0", req.QuorumRequired, req.QuorumReached)
	}
	if req.RequestID == "" {
		t.Error("expected generated request ID")
	}
	// 72h expiry window (FR-034).
	wantExp := req.OpenedAt.Add(72 * time.Hour)
	if req.ExpiresAt.Sub(wantExp).Abs() > time.Second {
		t.Errorf("expiry = %v, want ~%v", req.ExpiresAt, wantExp)
	}
}

func TestOpenDisclosure_ValidationErrors(t *testing.T) {
	svc := NewOversightService(newOversightDB(t))
	cases := []struct{ tx, requestor, reason string }{
		{"", "bank-a", "AML"},
		{"tx", "", "AML"},
		{"tx", "bank-a", ""},
	}
	for _, c := range cases {
		if _, err := svc.OpenDisclosure(context.Background(), c.tx, c.requestor, c.reason); err == nil {
			t.Errorf("tx=%q req=%q reason=%q: expected error", c.tx, c.requestor, c.reason)
		}
	}
}

func TestSignDisclosure_ReachesQuorum(t *testing.T) {
	db := newOversightDB(t)
	svc := NewOversightService(db)
	ctx := context.Background()
	req, err := svc.OpenDisclosure(ctx, "tx-1", "bank-a", "AML")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// First signature: quorum not yet reached.
	if err := svc.SignDisclosure(ctx, req.RequestID, "cb-1"); err != nil {
		t.Fatalf("first sign: %v", err)
	}
	status, _ := svc.GetDisclosureStatus(ctx, req.RequestID)
	if status.State != domain.DisclosurePending || status.QuorumReached != 1 {
		t.Errorf("after 1 sig: state=%q reached=%d, want PENDING/1", status.State, status.QuorumReached)
	}

	// Second signature from a different CB: quorum reached.
	if err := svc.SignDisclosure(ctx, req.RequestID, "cb-2"); err != nil {
		t.Fatalf("second sign: %v", err)
	}
	status, _ = svc.GetDisclosureStatus(ctx, req.RequestID)
	if status.State != domain.DisclosureQuorumReached || status.QuorumReached != 2 {
		t.Errorf("after 2 sigs: state=%q reached=%d, want QUORUM_REACHED/2", status.State, status.QuorumReached)
	}
}

func TestSignDisclosure_DuplicateSigner(t *testing.T) {
	db := newOversightDB(t)
	svc := NewOversightService(db)
	ctx := context.Background()
	req, _ := svc.OpenDisclosure(ctx, "tx-1", "bank-a", "AML")

	if err := svc.SignDisclosure(ctx, req.RequestID, "cb-1"); err != nil {
		t.Fatalf("first sign: %v", err)
	}
	// Same signer must be rejected — quorum integrity (FR-035).
	if err := svc.SignDisclosure(ctx, req.RequestID, "cb-1"); err == nil {
		t.Fatal("expected error for duplicate signer")
	}
	status, _ := svc.GetDisclosureStatus(ctx, req.RequestID)
	if status.QuorumReached != 1 {
		t.Errorf("quorum should stay at 1 after dup, got %d", status.QuorumReached)
	}
}

func TestSignDisclosure_NotFound(t *testing.T) {
	svc := NewOversightService(newOversightDB(t))
	if err := svc.SignDisclosure(context.Background(), "missing", "cb-1"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestSignDisclosure_Expired(t *testing.T) {
	db := newOversightDB(t)
	svc := NewOversightService(db)
	ctx := context.Background()
	// Insert an already-expired request directly.
	expired := &domain.DisclosureRequest{
		RequestID:            "exp-1",
		RequestedByBankID:    "bank-a",
		TargetTransactionRef: "tx",
		ReasonCode:           "AML",
		State:                domain.DisclosurePending,
		QuorumRequired:       2,
		OpenedAt:             time.Now().Add(-100 * time.Hour),
		ExpiresAt:            time.Now().Add(-time.Hour),
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.SignDisclosure(ctx, "exp-1", "cb-1"); err == nil {
		t.Fatal("expected expired error on sign")
	}
}

func TestGetDisclosureStatus_AutoExpireOnRead(t *testing.T) {
	db := newOversightDB(t)
	svc := NewOversightService(db)
	ctx := context.Background()
	stale := &domain.DisclosureRequest{
		RequestID:            "stale-1",
		RequestedByBankID:    "bank-a",
		TargetTransactionRef: "tx",
		ReasonCode:           "AML",
		State:                domain.DisclosurePending,
		QuorumRequired:       2,
		OpenedAt:             time.Now().Add(-100 * time.Hour),
		ExpiresAt:            time.Now().Add(-time.Hour),
	}
	db.Create(stale)

	got, err := svc.GetDisclosureStatus(ctx, "stale-1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.State != domain.DisclosureExpired {
		t.Errorf("expected auto-expire to EXPIRED, got %q", got.State)
	}
	// Persisted, too.
	var reload domain.DisclosureRequest
	db.First(&reload, "request_id = ?", "stale-1")
	if reload.State != domain.DisclosureExpired {
		t.Errorf("expected persisted EXPIRED, got %q", reload.State)
	}
}

func TestGetDisclosureStatus_NotFound(t *testing.T) {
	svc := NewOversightService(newOversightDB(t))
	if _, err := svc.GetDisclosureStatus(context.Background(), "nope"); err == nil {
		t.Fatal("expected not-found error")
	}
}
