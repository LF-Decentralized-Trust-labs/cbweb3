package repository

import (
	"context"
	"testing"
)

func TestMemoryParticipantsRepositoryUpsertAndGet(t *testing.T) {
	t.Parallel()

	repo := NewMemoryParticipantsRepository()
	err := repo.Upsert(context.Background(), Participant{
		UserID:         "bank-a",
		DID:            "did:example:bank-a",
		WalletAddress:  "0x1111111111111111111111111111111111111111",
		Country:        "BR",
		BankCode:       "001",
		Role:           "bank",
		SignerProvider: "local",
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	got, found, err := repo.GetByUser(context.Background(), "bank-a")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !found {
		t.Fatal("expected participant to be found")
	}
	if got.UserID != "bank-a" {
		t.Fatalf("unexpected participant: %+v", got)
	}
}

func TestMemoryParticipantsRepositoryRejectsEmptyUserID(t *testing.T) {
	t.Parallel()

	repo := NewMemoryParticipantsRepository()
	err := repo.Upsert(context.Background(), Participant{
		UserID: "",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
