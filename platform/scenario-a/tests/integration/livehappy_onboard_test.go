// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package integration_test

import (
	"testing"
	"time"
)

// onboardBank runs the PKI onboarding flow for a commercial bank through its home
// central bank. It is idempotent: if the bank is already ACTIVE it returns at once.
//
// Onboarding is OFF by default (ONBOARD=1 to enable): `make spoke-all` pre-registers
// the funded operators the happy path actually transacts with, so the core flow
// does not depend on a runtime onboarding round-trip. This helper exists for the
// opt-in case where the suite is run against a freshly nuked stack.
//
// Returns the on-chain participant-registration tx hash from the complete step —
// the CB's auth service registers the participant on its spoke (governance-signed)
// during /onboarding/complete and returns the tx hash there. approve-kyc performs
// no on-chain write, so its hash is always empty. Returns "" when the bank is
// already ACTIVE (no tx) or the response surfaced no hash — never fabricated.
func onboardBank(t *testing.T, bankClient, cbClient *httpClient, bankCode, institutionName, country string) string {
	t.Helper()

	// Fail cleanly (instead of nil-dereferencing) if a client wasn't logged in —
	// e.g. Phase1 login failed for this entity.
	if bankClient == nil || cbClient == nil {
		t.Fatalf("onboardBank[%s]: nil client — Phase1 login likely failed for this entity", bankCode)
	}

	var myStatus struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
		UserID    string `json:"user_id"`
	}
	_ = bankClient.get("/api/v1/onboarding/my-status?bank_code="+bankCode, &myStatus)
	if myStatus.Status == "ACTIVE" {
		t.Logf("  [%s] already ACTIVE (user_id=%s) — skipping onboarding", bankCode, myStatus.UserID)
		return ""
	}

	requestID, userID := myStatus.RequestID, myStatus.UserID
	if requestID == "" {
		var initiate struct {
			RequestID string `json:"request_id"`
			UserID    string `json:"user_id"`
		}
		bankClient.mustPost(t, "/api/v1/onboarding/initiate", map[string]string{
			"institution_name": institutionName,
			"bank_code":        bankCode,
			"country":          country,
			"role":             "commercial_bank",
			"email":            "admin@" + bankCode + ".test",
			"username":         bankCode + "-admin",
		}, &initiate)
		requestID, userID = initiate.RequestID, initiate.UserID
		t.Logf("  [%s] initiated onboarding: request_id=%s user_id=%s", bankCode, requestID, userID)
	} else {
		t.Logf("  [%s] resuming onboarding request id=%s status=%s", bankCode, requestID, myStatus.Status)
	}

	// CB approves KYC — idempotent; log but don't fail if already approved.
	// (No on-chain write here; the registration tx is minted at complete.)
	if err := cbClient.post("/api/v1/compliance/approve-kyc",
		map[string]string{"subject": userID, "reason": "integration-test"}, nil); err != nil {
		t.Logf("  [%s] approve-kyc: %v (may already be approved)", bankCode, err)
	}

	// Wait for the PoP nonce to be issued (KYC_APPROVED state).
	pollUntil(t, 3*time.Second, 30*time.Second, func() (bool, error) {
		var s struct {
			Status   string `json:"status"`
			PopNonce string `json:"pop_nonce"`
		}
		if err := bankClient.get("/api/v1/onboarding/status/"+requestID, &s); err != nil {
			return false, nil //nolint:nilerr
		}
		return s.PopNonce != "", nil
	})

	// Complete — the proxy signs the PoP nonce automatically. The CB's auth service
	// registers the participant on-chain here (governance-signed) and returns the
	// registration tx hash, which we fold into the evidence bundle.
	var done struct {
		WalletAddress string `json:"wallet_address"`
		Status        string `json:"status"`
		TxHash        string `json:"tx_hash"`
	}
	if err := bankClient.post("/api/v1/onboarding/complete",
		map[string]string{"request_id": requestID, "user_id": userID}, &done); err != nil {
		t.Logf("  [%s] complete: %v (may already be done)", bankCode, err)
		return ""
	}
	t.Logf("  [%s] onboarded: wallet=%s status=%s tx=%s", bankCode, done.WalletAddress, done.Status, done.TxHash)
	return done.TxHash
}
