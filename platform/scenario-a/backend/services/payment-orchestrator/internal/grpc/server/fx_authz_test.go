// SPDX-License-Identifier: Apache-2.0

// Server-side (orchestrator) authorization tests for the FX and HTLC mutating
// operations (R2-H-9 / R2-H-10). Unlike the gateway tests — which can only assert
// that a PermissionDenied is mapped to 403 — these exercise the real control: a
// non-party caller (identified via x-caller-identity) must be rejected by the
// orchestrator itself, and the party identity on a propose must be bound to the
// authenticated caller rather than trusted from the request.
package server_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	authzOriginatorIdentity   = "funded_operator@spoke-a-bank-a"
	authzCounterpartyIdentity = "funded_operator@spoke-b-bank-b"
	authzOriginatorBankID     = "bank-a"
	authzCounterpartyBankID   = "bank-b"
	authzStrangerBankID       = "bank-z"
)

// acceptByCounterparty drives a freshly-proposed agreement to ACCEPTED so the
// settle-authorization path can be exercised.
func acceptByCounterparty(t *testing.T, env *fxEnv, tradeID string) {
	t.Helper()
	if _, err := env.client.AcceptFXAgreement(
		ctxWithCallerIdentity(authzCounterpartyBankID),
		&pb.AcceptFXAgreementRequest{TradeId: tradeID},
	); err != nil {
		t.Fatalf("counterparty accept: %v", err)
	}
}

// TestFX_CancelByNonParty_Denied proves the orchestrator — not just the gateway —
// rejects a cancel from a caller that is not a party to the agreement, and admits a party.
func TestFX_CancelByNonParty_Denied(t *testing.T) {
	env := setupFXEnv(t, "")
	tid := proposeWithPaladinOriginator(t, env, "T-CANCEL-AUTHZ", authzOriginatorIdentity, authzCounterpartyIdentity)

	// A non-party (bank-z) must be denied.
	if _, err := env.client.CancelFXAgreement(
		ctxWithCallerIdentity(authzStrangerBankID),
		&pb.CancelFXAgreementRequest{TradeId: tid},
	); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-party cancel: expected PermissionDenied, got %v", err)
	}

	// A party (the originator) may cancel.
	if _, err := env.client.CancelFXAgreement(
		ctxWithCallerIdentity(authzOriginatorBankID),
		&pb.CancelFXAgreementRequest{TradeId: tid},
	); err != nil {
		t.Fatalf("party cancel: unexpected error: %v", err)
	}
}

// TestFX_SettleByNonParty_Denied proves the orchestrator rejects a settle from a
// non-party on an ACCEPTED agreement, and admits a party.
func TestFX_SettleByNonParty_Denied(t *testing.T) {
	env := setupFXEnv(t, "")
	tid := proposeWithPaladinOriginator(t, env, "T-SETTLE-AUTHZ", authzOriginatorIdentity, authzCounterpartyIdentity)
	acceptByCounterparty(t, env, tid)

	// A non-party (bank-z) must be denied.
	if _, err := env.client.SettleFXAgreement(
		ctxWithCallerIdentity(authzStrangerBankID),
		&pb.SettleFXAgreementRequest{TradeId: tid},
	); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-party settle: expected PermissionDenied, got %v", err)
	}

	// A party (the counterparty) may settle.
	if _, err := env.client.SettleFXAgreement(
		ctxWithCallerIdentity(authzCounterpartyBankID),
		&pb.SettleFXAgreementRequest{TradeId: tid},
	); err != nil {
		t.Fatalf("party settle: unexpected error: %v", err)
	}
}

// TestFX_CancelSettle_NoCallerIdentity_Skipped confirms that a trusted internal/
// relay caller (no x-caller-identity — authenticated by the transport layer) is not
// blocked by the party check, preserving relay-driven flows.
func TestFX_CancelSettle_NoCallerIdentity_Skipped(t *testing.T) {
	env := setupFXEnv(t, "")
	tid := proposeWithPaladinOriginator(t, env, "T-NOID-AUTHZ", authzOriginatorIdentity, authzCounterpartyIdentity)

	// No x-caller-identity in context → check is skipped, cancel proceeds.
	if _, err := env.client.CancelFXAgreement(context.Background(), &pb.CancelFXAgreementRequest{TradeId: tid}); err != nil {
		t.Fatalf("relay/internal cancel (no identity): unexpected error: %v", err)
	}
}

// TestFX_ProposeForeignOriginator_Denied proves the orchestrator binds the
// originator to the authenticated caller: a caller may not propose naming a foreign
// bank as originator (which would otherwise let them "accept" their own trade).
func TestFX_ProposeForeignOriginator_Denied(t *testing.T) {
	env := setupFXEnv(t, "")

	base := func(originator string) *pb.ProposeFXAgreementRequest {
		return &pb.ProposeFXAgreementRequest{
			TradeId:         "T-PROP-" + originator,
			Originator:      originator,
			CounterpartyB:   authzCounterpartyIdentity,
			OriginAmount:    "100",
			CounterAmount:   "120",
			OriginCurrency:  "USD",
			CounterCurrency: "BRL",
			Rate:            "1.2",
			ExpiryDate:      uint64(time.Now().Add(time.Hour).Unix()),
			SourceSpokeId:   "spoke-a",
			DestSpokeId:     "spoke-b",
			SourceReceiver:  "recv@spoke-a-bank-a",
			DestReceiver:    "recv@spoke-b-bank-b",
		}
	}

	// Caller bank-a naming bank-b as originator → denied.
	if _, err := env.client.ProposeFXAgreement(
		ctxWithCallerIdentity(authzOriginatorBankID),
		base("op@spoke-b-bank-b"),
	); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("foreign originator propose: expected PermissionDenied, got %v", err)
	}

	// Caller bank-a naming its own identity as originator → allowed.
	if _, err := env.client.ProposeFXAgreement(
		ctxWithCallerIdentity(authzOriginatorBankID),
		base(authzOriginatorIdentity),
	); err != nil {
		t.Fatalf("matching originator propose: unexpected error: %v", err)
	}
}
