// SPDX-License-Identifier: Apache-2.0

// Package bootstrap handles automatic provisioning of the Central Bank
// governance participant on service startup.
package bootstrap

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

// EnsureGovernanceParticipant registers the Central Bank as a governance
// participant on first startup. It is idempotent: if a participant with the
// configured user ID already exists in the database, the function returns
// immediately without making any changes.
//
// Required environment variables:
//
//	GOVERNANCE_USER_ID — Keycloak subject for the CB service account
//	                     (default: "service-account-cbweb3-auth")
//	CB_PRIVATE_KEY     — secp256k1 hex private key used to derive wallet address
func EnsureGovernanceParticipant(
	ctx context.Context,
	repo repository.Repository,
	bc registry.RegistryWriter,
	ca *compliancepki.CA,
) error {
	cbKey := os.Getenv("CB_PRIVATE_KEY")
	if strings.TrimSpace(cbKey) == "" {
		log.Println("bootstrap: CB_PRIVATE_KEY not set — skipping governance bootstrap (commercial bank mode)")
		return nil
	}

	userID := getEnv("GOVERNANCE_USER_ID", "service-account-cbweb3-auth")

	if _, found, _ := repo.GetParticipantByUser(ctx, userID); found {
		log.Printf("bootstrap: governance participant '%s' already exists — skipping", userID)
		return nil
	}

	walletAddr, err := deriveWalletAddress(cbKey)
	if err != nil {
		return fmt.Errorf("bootstrap: derive wallet: %w", err)
	}

	// 3. Upsert participant with ROLE_GOVERNANCE and ACTIVE status.
	certPEM := ""
	if ca != nil {
		certPEM = ca.CertPEM()
	}
	if err := repo.UpsertParticipant(ctx, repository.Participant{
		UserID:          userID,
		InstitutionName: "Banco Central",
		Role:            "ROLE_GOVERNANCE",
		Status:          "ACTIVE",
		WalletAddress:   walletAddr,
		CertificateData: certPEM,
	}); err != nil {
		return fmt.Errorf("bootstrap: upsert governance participant: %w", err)
	}

	// 4. Register + verify on-chain (best-effort — never fails startup). Two-step onboarding
	// (R1-10.6 / R2-10.6): registerParticipant alone would leave the CB in Pending, so canGovern
	// would be false and every governance-gated call would revert. EnsureVerifiedParticipant
	// completes the Pending->Verified promotion and is idempotent across restarts (no demotion).
	if _, err := registry.EnsureVerifiedParticipant(ctx, bc, walletAddr, "Banco Central", "ROLE_GOVERNANCE", [32]byte{}); err != nil {
		log.Printf("bootstrap: on-chain register+verify failed (non-fatal): %v", err)
	}

	// 5. Audit log.
	_ = repo.CreateAuditLog(ctx, repository.AuditEntry{
		ActorSubject: userID,
		ActorAddress: walletAddr,
		ActionType:   "GOVERNANCE_BOOTSTRAP",
		Result:       "SUCCESS",
		Category:     "CREDENTIAL",
		Severity:     "INFO",
		Details:      `{"source":"startup_bootstrap"}`,
	})

	log.Printf("bootstrap: governance participant '%s' registered with wallet %s", userID, walletAddr)
	return nil
}

// deriveWalletAddress converts a hex-encoded secp256k1 private key into its
// Ethereum-style wallet address (keccak256 of public key → last 20 bytes).
func deriveWalletAddress(privKeyHex string) (string, error) {
	if strings.TrimSpace(privKeyHex) == "" {
		return "", fmt.Errorf("private key hex is empty")
	}
	b, err := hex.DecodeString(strings.TrimPrefix(privKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid hex in CB_PRIVATE_KEY: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(b)
	if err != nil {
		return "", fmt.Errorf("invalid ECDSA key in CB_PRIVATE_KEY: %w", err)
	}
	return gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex(), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
