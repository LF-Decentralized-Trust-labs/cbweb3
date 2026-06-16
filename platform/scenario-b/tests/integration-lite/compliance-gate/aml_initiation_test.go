// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// Suite 1 — AML/CFT failure-at-initiation (D6 integration priority).
//
// Validates the constitution's compliance-gate-at-initiation requirement: a
// cross-currency swap whose payer/beneficiary lack a valid, non-expired
// ZK-Pointer (unverified identity / revoked / expired) MUST be rejected AT
// INITIATION, before any AMM swap or bridge mint executes. The gate is re-checked
// at payment initiation, not merely at onboarding.
//
// Wiring (all in-process, hermetic):
//   - compliance services.ZKComplianceGate          (REAL gate, in-memory SQLite zk-pointer table)
//   - compliance grpc/server.New over bufconn        (REAL gRPC server; supplies the circuit-breaker state)
//   - swapInitiator                                  (mirrors api-gateway SwapService.Execute gate ordering:
//                                                     breaker → ZK compliance → submit)
//   - spyAMM                                          (records whether a swap / downstream mint ever fired)
package integrationlite

import (
	"context"
	"errors"
	"testing"
	"time"

	compsvc "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/services"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

// zkPointerRow mirrors the columns the REAL compliance ZK gate queries on
// compliance_zk_pointers (the gate reads a raw table, not a gorm model).
type zkPointerRow struct {
	PointerID      string `gorm:"column:pointer_id;primaryKey"`
	BankID         string `gorm:"column:bank_id"`
	CommitmentHash string `gorm:"column:commitment_hash"`
	State          string `gorm:"column:state"`
	ExpiresAt      *time.Time
}

func (zkPointerRow) TableName() string { return "compliance_zk_pointers" }

func seedZKPointer(t *testing.T, db *gorm.DB, row zkPointerRow) {
	t.Helper()
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed zk pointer: %v", err)
	}
}

// spyAMM records swap submissions so tests can assert no value left the gate when
// compliance / breaker rejects.
type spyAMM struct {
	swaps int
}

func (a *spyAMM) submit() (string, error) {
	a.swaps++
	return "order-1", nil
}

// swapError marks a denied swap; Stage records which gate stopped it.
type swapError struct {
	Stage string
	Err   error
}

func (e *swapError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *swapError) Unwrap() error { return e.Err }

// swapRequest mirrors the load-bearing fields of api-gateway services.SwapRequest.
type swapRequest struct {
	Pair                 string
	PayerID, BeneficiaryID string
	ZKPayer, ZKBeneficiary string
}

// swapInitiator reproduces the GATE ORDERING of api-gateway SwapService.Execute
// (circuit-breaker → ZK compliance → submit), but drives the REAL compliance ZK
// gate and the REAL compliance gRPC circuit-breaker state. A denial at any gate
// must occur BEFORE the AMM submit (no mint on a failed compliance check).
type swapInitiator struct {
	gate    *compsvc.ZKComplianceGate
	breaker compliancv1.ComplianceServiceClient
	amm     *spyAMM
}

func (s *swapInitiator) Execute(ctx context.Context, req swapRequest) (string, error) {
	// 1. Circuit-breaker gate (FR-030) — sourced from the REAL compliance service.
	st, err := s.breaker.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return "", &swapError{Stage: "circuit_breaker", Err: err}
	}
	if st.IsPaused {
		return "", &swapError{Stage: "circuit_breaker", Err: errors.New("circuit breaker paused")}
	}

	// 2. ZK-Pointer compliance gate (FR-025 / FR-058), re-checked AT INITIATION.
	if err := s.gate.ValidateZKPointer(ctx, req.PayerID, "", req.ZKPayer); err != nil {
		return "", &swapError{Stage: "zk_payer", Err: err}
	}
	if err := s.gate.ValidateZKPointer(ctx, req.BeneficiaryID, "", req.ZKBeneficiary); err != nil {
		return "", &swapError{Stage: "zk_beneficiary", Err: err}
	}

	// 3. Only now may the swap / downstream bridge mint proceed.
	return s.amm.submit()
}

type env struct {
	zkDB *gorm.DB
	comp *complianceEnv
	init *swapInitiator
	amm  *spyAMM
}

func newEnv(t *testing.T) *env {
	t.Helper()
	zkDB := newMemDB(t, &zkPointerRow{})
	comp := newComplianceEnv(t)
	amm := &spyAMM{}
	return &env{
		zkDB: zkDB,
		comp: comp,
		amm:  amm,
		init: &swapInitiator{
			gate:    compsvc.NewZKComplianceGate(zkDB),
			breaker: comp.client,
			amm:     amm,
		},
	}
}

func validRequest() swapRequest {
	return swapRequest{
		Pair: "W-BRL-ARS", PayerID: "bank-a", BeneficiaryID: "bank-b",
		ZKPayer: "0xpayer", ZKBeneficiary: "0xbeneficiary",
	}
}

func assertSwapError(t *testing.T, err error, wantStage string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected swap to be REJECTED")
	}
	var se *swapError
	if !errors.As(err, &se) {
		t.Fatalf("expected swapError, got %T: %v", err, err)
	}
	if se.Stage != wantStage {
		t.Fatalf("expected rejection at stage %q, got %q (%v)", wantStage, se.Stage, err)
	}
}

// TestAML_UnverifiedPayer_RejectedAtInitiation: payer has no ZK-Pointer at all.
func TestAML_UnverifiedPayer_RejectedAtInitiation(t *testing.T) {
	e := newEnv(t)
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-bene", BankID: "bank-b", CommitmentHash: "0xbeneficiary", State: "VALID"})

	_, err := e.init.Execute(context.Background(), validRequest())
	assertSwapError(t, err, "zk_payer")
	if e.amm.swaps != 0 {
		t.Errorf("AML denial must prevent AMM swap; got %d swaps", e.amm.swaps)
	}
}

// TestAML_RevokedBeneficiary_RejectedAtInitiation: beneficiary pointer is REVOKED.
func TestAML_RevokedBeneficiary_RejectedAtInitiation(t *testing.T) {
	e := newEnv(t)
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-payer", BankID: "bank-a", CommitmentHash: "0xpayer", State: "VALID"})
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-bene", BankID: "bank-b", CommitmentHash: "0xbeneficiary", State: "REVOKED"})

	_, err := e.init.Execute(context.Background(), validRequest())
	assertSwapError(t, err, "zk_beneficiary")
	if e.amm.swaps != 0 {
		t.Errorf("revoked beneficiary must prevent swap; got %d swaps", e.amm.swaps)
	}
}

// TestAML_ExpiredPointer_RejectedAtInitiation asserts the "re-check at initiation,
// not just onboarding" rule: a pointer that was VALID at onboarding but has since
// expired must be denied at swap time.
func TestAML_ExpiredPointer_RejectedAtInitiation(t *testing.T) {
	e := newEnv(t)
	past := time.Now().Add(-time.Hour)
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-payer", BankID: "bank-a", CommitmentHash: "0xpayer", State: "VALID", ExpiresAt: &past})
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-bene", BankID: "bank-b", CommitmentHash: "0xbeneficiary", State: "VALID"})

	_, err := e.init.Execute(context.Background(), validRequest())
	assertSwapError(t, err, "zk_payer")
	if e.amm.swaps != 0 {
		t.Errorf("expired pointer must prevent swap; got %d swaps", e.amm.swaps)
	}
}

// TestAML_BothVerified_SwapProceeds is the positive control: with both parties
// holding VALID, non-expired pointers and the breaker LIVE, the gate allows the
// swap and the AMM is reached exactly once — proving the denials above are caused
// by the gate, not an always-failing pipeline.
func TestAML_BothVerified_SwapProceeds(t *testing.T) {
	e := newEnv(t)
	future := time.Now().Add(time.Hour)
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-payer", BankID: "bank-a", CommitmentHash: "0xpayer", State: "VALID", ExpiresAt: &future})
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-bene", BankID: "bank-b", CommitmentHash: "0xbeneficiary", State: "VALID", ExpiresAt: &future})

	orderID, err := e.init.Execute(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("expected swap to proceed for two verified parties: %v", err)
	}
	if orderID == "" {
		t.Fatal("expected a swap order id")
	}
	if e.amm.swaps != 1 {
		t.Errorf("expected exactly 1 AMM swap, got %d", e.amm.swaps)
	}
}

// TestAML_CircuitBreakerPaused_BlocksSwap drives the REAL compliance gRPC server:
// pause the breaker via ToggleCircuitBreaker, then assert the initiation pipeline
// is blocked at the breaker stage before the ZK gate / AMM are reached — even for
// otherwise-verified parties (defense in depth at initiation, FR-030).
func TestAML_CircuitBreakerPaused_BlocksSwap(t *testing.T) {
	e := newEnv(t)
	future := time.Now().Add(time.Hour)
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-payer", BankID: "bank-a", CommitmentHash: "0xpayer", State: "VALID", ExpiresAt: &future})
	seedZKPointer(t, e.zkDB, zkPointerRow{PointerID: "p-bene", BankID: "bank-b", CommitmentHash: "0xbeneficiary", State: "VALID", ExpiresAt: &future})

	// Pause the circuit breaker via the REAL compliance gRPC service (1-of-N pause).
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-actor-subject", "supervisor-1")
	if _, err := e.comp.client.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{
		Pause:  true,
		Reason: "AML incident — halt swaps",
	}); err != nil {
		t.Fatalf("ToggleCircuitBreaker: %v", err)
	}

	_, err := e.init.Execute(context.Background(), validRequest())
	assertSwapError(t, err, "circuit_breaker")
	if e.amm.swaps != 0 {
		t.Errorf("paused breaker must prevent swap; got %d swaps", e.amm.swaps)
	}
}
