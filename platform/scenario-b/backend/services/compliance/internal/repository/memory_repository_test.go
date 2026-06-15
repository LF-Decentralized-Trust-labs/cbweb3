// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"
)

func TestMemoryRepository_ParticipantCRUD(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	if err := repo.UpsertParticipant(ctx, Participant{UserID: ""}); err == nil {
		t.Fatal("expected error for empty userID")
	}

	if err := repo.UpsertParticipant(ctx, Participant{UserID: "u1", BankCode: "AAA", Status: "ACTIVE", InstitutionName: "Bank A"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// update path
	if err := repo.UpsertParticipant(ctx, Participant{UserID: "u1", BankCode: "AAA", Status: "FROZEN", InstitutionName: "Bank A"}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	p, found, err := repo.GetParticipantByUser(ctx, "u1")
	if err != nil || !found || p.Status != "FROZEN" {
		t.Fatalf("get: p=%+v found=%v err=%v", p, found, err)
	}

	_, found, _ = repo.GetParticipantByUser(ctx, "ghost")
	if found {
		t.Fatal("expected not found")
	}
}

func TestMemoryRepository_ListParticipants(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	_ = repo.UpsertParticipant(ctx, Participant{UserID: "u1", BankCode: "AAA", Status: "ACTIVE"})
	_ = repo.UpsertParticipant(ctx, Participant{UserID: "u2", BankCode: "BBB", Status: "FROZEN"})

	all, _ := repo.ListParticipants(ctx, ParticipantFilter{})
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}

	byStatus, _ := repo.ListParticipants(ctx, ParticipantFilter{Status: "ACTIVE"})
	if len(byStatus) != 1 || byStatus[0].UserID != "u1" {
		t.Fatalf("status filter: %+v", byStatus)
	}

	byBank, _ := repo.ListParticipants(ctx, ParticipantFilter{BankCode: "BBB"})
	if len(byBank) != 1 || byBank[0].UserID != "u2" {
		t.Fatalf("bank filter: %+v", byBank)
	}
}

func TestMemoryRepository_AuditLogs(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	_ = repo.CreateAuditLog(ctx, AuditEntry{ActionType: "A", Category: "SESSION", Severity: "INFO"})
	_ = repo.CreateAuditLog(ctx, AuditEntry{ActionType: "B", Category: "FREEZE", Severity: "CRITICAL"})

	all, _ := repo.GetAuditLogs(ctx, AuditFilter{})
	if len(all) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(all))
	}

	freeze, _ := repo.GetAuditLogs(ctx, AuditFilter{Category: "FREEZE"})
	if len(freeze) != 1 || freeze[0].ActionType != "B" {
		t.Fatalf("category filter: %+v", freeze)
	}

	crit, _ := repo.GetAuditLogs(ctx, AuditFilter{Severity: "CRITICAL"})
	if len(crit) != 1 {
		t.Fatalf("severity filter: %+v", crit)
	}

	// limit honored
	limited, _ := repo.GetAuditLogs(ctx, AuditFilter{Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("limit: %+v", limited)
	}
}

func TestMemoryRepository_SystemParameters(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	_, found, _ := repo.GetSystemParameter(ctx, "missing")
	if found {
		t.Fatal("expected missing param")
	}

	if err := repo.UpsertSystemParameter(ctx, SystemParameter{Key: "k", Value: "v1", UpdatedBy: "a"}); err != nil {
		t.Fatalf("upsert param: %v", err)
	}
	v, found, _ := repo.GetSystemParameter(ctx, "k")
	if !found || v != "v1" {
		t.Fatalf("get param: v=%q found=%v", v, found)
	}

	// update
	_ = repo.UpsertSystemParameter(ctx, SystemParameter{Key: "k", Value: "v2", UpdatedBy: "b"})
	v, _, _ = repo.GetSystemParameter(ctx, "k")
	if v != "v2" {
		t.Fatalf("expected v2, got %q", v)
	}
}
