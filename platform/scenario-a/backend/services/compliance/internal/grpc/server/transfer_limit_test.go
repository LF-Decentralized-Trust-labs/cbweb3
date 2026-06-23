// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestService builds a complianceService backed by an in-memory repository
// with no CA and no blockchain writer — sufficient for transfer-limit tests.
func newTestService() *complianceService {
	return &complianceService{repo: repository.NewMemoryRepository()}
}

// ── humanToWei ────────────────────────────────────────────────────────────────

func TestHumanToWei(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		wantWei  string
		wantErr  bool
	}{
		{"1", "1000000000000000000", false},
		{"0.5", "500000000000000000", false},
		{"1000000", "1000000000000000000000000", false},
		{"0.000000000000000001", "1", false},
		{"", "", true},
		{"abc", "", true},
		{"1.abc", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := humanToWei(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got.String() != tc.wantWei {
				t.Errorf("humanToWei(%q) = %q, want %q", tc.input, got.String(), tc.wantWei)
			}
		})
	}
}

// ── centralBankIDForPayer ─────────────────────────────────────────────────────

func TestCentralBankIDForPayer(t *testing.T) {
	t.Parallel()

	cases := []struct{ payer, want string }{
		{"bank-a", "central-bank-a"},
		{"BANK-A", "central-bank-a"},
		{"bank-b", "central-bank-b"},
		{"spoke-b", "central-bank-b"},
		{"bank-c", ""},
		{"", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.payer, func(t *testing.T) {
			t.Parallel()
			if got := centralBankIDForPayer(tc.payer); got != tc.want {
				t.Errorf("centralBankIDForPayer(%q) = %q, want %q", tc.payer, got, tc.want)
			}
		})
	}
}

// ── CreateTransferLimit ───────────────────────────────────────────────────────

func TestCreateTransferLimit_Success(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	resp, err := svc.CreateTransferLimit(context.Background(), &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "500000",
		ActorSubject:  "treasury-user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Limit == nil {
		t.Fatal("expected non-nil limit in response")
	}
	if resp.Limit.LimitId == "" {
		t.Error("expected non-empty limit_id")
	}
	if resp.Limit.MaxAmount != "500000" {
		t.Errorf("max_amount = %q, want %q", resp.Limit.MaxAmount, "500000")
	}
	if !resp.Limit.IsActive {
		t.Error("new limit should be active")
	}
}

func TestCreateTransferLimit_MissingMaxAmount(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	_, err := svc.CreateTransferLimit(context.Background(), &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateTransferLimit_InvalidMaxAmount(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	_, err := svc.CreateTransferLimit(context.Background(), &compliancv1.CreateTransferLimitRequest{
		MaxAmount: "not-a-number",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// ── ListTransferLimits ────────────────────────────────────────────────────────

func TestListTransferLimits_Empty(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	resp, err := svc.ListTransferLimits(context.Background(), &compliancv1.ListTransferLimitsRequest{
		CentralBankId: "cb-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Limits) != 0 {
		t.Errorf("expected empty limits, got %d", len(resp.Limits))
	}
}

func TestListTransferLimits_AfterCreate(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	_, _ = svc.CreateTransferLimit(context.Background(), &compliancv1.CreateTransferLimitRequest{
		MaxAmount: "1000",
	})

	resp, err := svc.ListTransferLimits(context.Background(), &compliancv1.ListTransferLimitsRequest{
		CentralBankId: "central-bank-a", // centralBankIDFromCtx fallback
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Limits) != 1 {
		t.Errorf("expected 1 limit, got %d", len(resp.Limits))
	}
}

// ── DeleteTransferLimit ───────────────────────────────────────────────────────

func TestDeleteTransferLimit_Success(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	create, _ := svc.CreateTransferLimit(context.Background(), &compliancv1.CreateTransferLimitRequest{MaxAmount: "1000"})
	limitID := create.Limit.LimitId

	if _, err := svc.DeleteTransferLimit(context.Background(), &compliancv1.DeleteTransferLimitRequest{
		LimitId: limitID, ActorSubject: "treasury",
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Limit must no longer appear in list.
	listResp, _ := svc.ListTransferLimits(context.Background(), &compliancv1.ListTransferLimitsRequest{
		CentralBankId: "central-bank-a",
	})
	for _, l := range listResp.Limits {
		if l.LimitId == limitID {
			t.Error("deleted limit still visible in list")
		}
	}
}

func TestDeleteTransferLimit_MissingID(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	_, err := svc.DeleteTransferLimit(context.Background(), &compliancv1.DeleteTransferLimitRequest{LimitId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// ── CheckAndDeductTransferLimit ───────────────────────────────────────────────

func TestCheckAndDeductTransferLimit_NoLimitConfigured_Allowed(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	resp, err := svc.CheckAndDeductTransferLimit(context.Background(), &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a",
		Currency:    "BRL",
		AmountHuman: "100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Error("expected allowed=true when no limit configured")
	}
}

func TestCheckAndDeductTransferLimit_WithinLimit_Allowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newTestService()

	_, _ = svc.CreateTransferLimit(ctx, &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "1000",
	})

	resp, err := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a",
		Currency:    "BRL",
		AmountHuman: "500",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Errorf("expected allowed=true for amount within limit, error_code=%q", resp.ErrorCode)
	}
}

func TestCheckAndDeductTransferLimit_ExceedsLimit_Rejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newTestService()

	_, _ = svc.CreateTransferLimit(ctx, &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "100",
	})

	resp, err := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a",
		Currency:    "BRL",
		AmountHuman: "101",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Error("expected allowed=false when amount exceeds limit")
	}
	if resp.ErrorCode != "TRANSFER_LIMIT_EXCEEDED" {
		t.Errorf("error_code = %q, want TRANSFER_LIMIT_EXCEEDED", resp.ErrorCode)
	}
	if resp.MaxAmount != "100" {
		t.Errorf("max_amount = %q, want '100'", resp.MaxAmount)
	}
}

func TestCheckAndDeductTransferLimit_AccumulatesAcrossCalls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newTestService()

	_, _ = svc.CreateTransferLimit(ctx, &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "1000",
	})

	// Two transfers of 600 each — second must be rejected (600+600 > 1000).
	r1, _ := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a", Currency: "BRL", AmountHuman: "600",
	})
	if !r1.Allowed {
		t.Fatalf("first transfer should be allowed")
	}

	r2, _ := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a", Currency: "BRL", AmountHuman: "600",
	})
	if r2.Allowed {
		t.Error("second transfer should be rejected (cumulative limit exceeded)")
	}
}

func TestCheckAndDeductTransferLimit_MissingFields(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	_, err := svc.CheckAndDeductTransferLimit(context.Background(), &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "",
		AmountHuman: "",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestCheckAndDeductTransferLimit_UnknownSuffix_Allowed(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	// payer_bank_id with neither -a nor -b suffix → no CB → allowed (no enforcement).
	resp, err := svc.CheckAndDeductTransferLimit(context.Background(), &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-c",
		Currency:    "USD",
		AmountHuman: "99999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Error("expected allowed=true for unknown payer suffix (no CB mapping)")
	}
}

// ── RestoreTransferLimit ──────────────────────────────────────────────────────

func TestRestoreTransferLimit_ReducesVolume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := newTestService()

	_, _ = svc.CreateTransferLimit(ctx, &compliancv1.CreateTransferLimitRequest{
		ParticipantId: "bank-a",
		Currency:      "BRL",
		MaxAmount:     "1000",
	})

	// Deduct 800, then restore 300 — next check for 600 should be allowed (800-300+600=1100 > 1000? no: 500+600=1100 > 1000).
	// Actually: deduct 800, restore 300 → accumulated=500, so 600 > 1000-500=500 → rejected.
	// Let's use simpler numbers: deduct 900, restore 500 → accumulated=400, 400+500=900 ≤ 1000 → allowed.
	r1, _ := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a", Currency: "BRL", AmountHuman: "900",
	})
	if !r1.Allowed {
		t.Fatal("initial deduct should be allowed")
	}

	if _, err := svc.RestoreTransferLimit(ctx, &compliancv1.RestoreTransferLimitRequest{
		PayerBankId: "bank-a", Currency: "BRL", AmountHuman: "500",
	}); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Accumulated is now 400 (900-500); 400+500=900 ≤ 1000 → should be allowed.
	r2, _ := svc.CheckAndDeductTransferLimit(ctx, &compliancv1.CheckAndDeductTransferLimitRequest{
		PayerBankId: "bank-a", Currency: "BRL", AmountHuman: "500",
	})
	if !r2.Allowed {
		t.Error("expected allowed after restore reduced accumulated volume")
	}
}

func TestRestoreTransferLimit_BestEffort_EmptyFields(t *testing.T) {
	t.Parallel()
	svc := newTestService()

	// Best-effort: empty fields must not error.
	if _, err := svc.RestoreTransferLimit(context.Background(), &compliancv1.RestoreTransferLimitRequest{
		PayerBankId: "",
		AmountHuman: "",
	}); err != nil {
		t.Errorf("restore with empty fields should be best-effort (no error), got %v", err)
	}
}
