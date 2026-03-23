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
	userID := getEnv("GOVERNANCE_USER_ID", "service-account-cbweb3-auth")

	// 1. Already registered — nothing to do.
	if _, found, _ := repo.GetParticipantByUser(ctx, userID); found {
		log.Printf("bootstrap: governance participant '%s' already exists — skipping", userID)
		return nil
	}

	// 2. Derive wallet address from CB_PRIVATE_KEY.
	walletAddr, err := deriveWalletAddress(os.Getenv("CB_PRIVATE_KEY"))
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

	// 4. Register on-chain (best-effort — never fails startup).
	if _, err := bc.SetParticipant(ctx, walletAddr, "ROLE_GOVERNANCE", true); err != nil {
		log.Printf("bootstrap: on-chain registration failed (non-fatal): %v", err)
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
		return "", fmt.Errorf("CB_PRIVATE_KEY is not set")
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
