// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// onboardBank runs the full 4-step PKI onboarding for a commercial bank.
// It is idempotent: if the bank already has status ACTIVE it returns immediately.
// bankClient is the bank's API gateway client; cbClient is its home central-bank's client.
// Returns the user_id of the onboarded participant and the on-chain
// participant-registration tx hash from the complete step — the CB's auth service
// registers the participant on its spoke (governance-signed) during
// /onboarding/complete and returns the tx hash there. approve-kyc performs no
// on-chain write, so its hash is always empty. Returns "" for the hash when the
// bank was already ACTIVE or the response surfaced none. Callers feed the hash into
// the evidence recorder as a txRef so phase_2 captures the participant-registration tx.
func onboardBank(t *testing.T, bankClient, cbClient *httpClient, bankCode, institutionName, country string) (userID, kycTxHash string) {
	t.Helper()

	// Step 0: check existing state — resume if we have a partial request.
	var myStatus struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
		UserID    string `json:"user_id"`
	}
	_ = bankClient.get(t, "/api/v1/onboarding/my-status?bank_code="+bankCode, &myStatus)

	if myStatus.Status == "ACTIVE" {
		// Idempotent skip: no new registration tx is produced, so record no ref.
		t.Logf("  [%s] already ACTIVE (user_id=%s) — skipping", bankCode, myStatus.UserID)
		return myStatus.UserID, ""
	}

	var requestID string
	if myStatus.RequestID != "" {
		t.Logf("  [%s] resuming existing request: id=%s status=%s", bankCode, myStatus.RequestID, myStatus.Status)
		requestID = myStatus.RequestID
		userID = myStatus.UserID
	} else {
		// Initiate a new credential request.
		var initiateResp struct {
			RequestID string `json:"request_id"`
			UserID    string `json:"user_id"`
		}
		require.NoError(t,
			bankClient.post(t, "/api/v1/onboarding/initiate", map[string]string{
				"institution_name": institutionName,
				"bank_code":        bankCode,
				"country":          country,
				"role":             "commercial_bank",
				"email":            "admin@" + bankCode + ".test",
				"username":         bankCode + "-admin",
			}, &initiateResp),
			"onboarding initiate for "+bankCode,
		)
		requestID = initiateResp.RequestID
		userID = initiateResp.UserID
		t.Logf("  [%s] initiated: request_id=%s user_id=%s", bankCode, requestID, userID)
	}

	// Step 2: CB approves KYC. Idempotent — log but don't fail if already approved.
	// (No on-chain write here; the registration tx is minted at complete.)
	if err := cbClient.post(t, "/api/v1/compliance/approve-kyc",
		map[string]string{"subject": userID, "reason": "integration-test"},
		nil,
	); err != nil {
		t.Logf("  [%s] approve-kyc: %v (may already be approved)", bankCode, err)
	}

	// Step 3: Poll until pop_nonce is available (KYC_APPROVED state).
	pollUntil(t, 3*time.Second, 30*time.Second, func() (bool, error) {
		var s struct {
			Status   string `json:"status"`
			PopNonce string `json:"pop_nonce"`
		}
		if err := bankClient.get(t, "/api/v1/onboarding/status/"+requestID, &s); err != nil {
			return false, nil
		}
		t.Logf("  [%s] status=%s pop_nonce_present=%v", bankCode, s.Status, s.PopNonce != "")
		return s.PopNonce != "", nil
	})

	// Step 4: Complete onboarding — proxy signs the PoP nonce automatically. The CB's
	// auth service registers the participant on-chain here (governance-signed) and
	// returns the registration tx hash, which we fold into the evidence bundle.
	var completeResp struct {
		UserID        string `json:"user_id"`
		WalletAddress string `json:"wallet_address"`
		Status        string `json:"status"`
		TxHash        string `json:"tx_hash"`
	}
	if err := bankClient.post(t, "/api/v1/onboarding/complete", map[string]string{
		"request_id": requestID,
		"user_id":    userID,
	}, &completeResp); err != nil {
		t.Logf("  [%s] complete: %v (may already be done)", bankCode, err)
	} else {
		kycTxHash = completeResp.TxHash
		t.Logf("  [%s] onboarded: wallet=%s status=%s tx=%s", bankCode, completeResp.WalletAddress, completeResp.Status, completeResp.TxHash)
		if completeResp.UserID != "" {
			userID = completeResp.UserID
		}
	}

	return userID, kycTxHash
}
