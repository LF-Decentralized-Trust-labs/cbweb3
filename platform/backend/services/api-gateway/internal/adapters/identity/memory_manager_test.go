// This file tests in-memory wallet-to-user binding constraints and conflicts.
package identity

import "testing"

func TestMemoryIdentityManagerBindAndRead(t *testing.T) {
	t.Parallel()

	manager := NewMemoryIdentityManager()
	binding, err := manager.BindWallet("bank-a", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("unexpected bind error: %v", err)
	}
	if binding.UserID != "bank-a" {
		t.Fatalf("unexpected user id: %s", binding.UserID)
	}

	read, ok := manager.GetByUser("bank-a")
	if !ok {
		t.Fatal("expected binding to exist")
	}
	if read.WalletAddress == "" {
		t.Fatal("expected wallet address")
	}
}

func TestMemoryIdentityManagerConflict(t *testing.T) {
	t.Parallel()

	manager := NewMemoryIdentityManager()
	wallet := "0x1111111111111111111111111111111111111111"

	if _, err := manager.BindWallet("bank-a", wallet); err != nil {
		t.Fatalf("unexpected bind error: %v", err)
	}
	if _, err := manager.BindWallet("bank-b", wallet); err == nil {
		t.Fatal("expected conflict for already bound wallet")
	}
}

