// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net"
	"testing"
	"time"

	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- bufconn harness ---

func newTestClient(t *testing.T, repo repository.Repository, ca *compliancepki.CA) (compliancv1.ComplianceServiceClient, context.Context) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := New(repo, ca, nil) // nil bc -> NoopRegistryClient
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return compliancv1.NewComplianceServiceClient(conn), context.Background()
}

func testCA(t *testing.T) *compliancepki.CA {
	t.Helper()
	certPEM, keyPEM, err := pki.GenerateSelfSignedCA("Test CB CA", "Central Bank", 1)
	if err != nil {
		t.Fatalf("gen CA: %v", err)
	}
	return compliancepki.NewCAFromPEM(certPEM, keyPEM)
}

func codeOf(err error) codes.Code { return status.Code(err) }

// --- Participant ---

func TestUpsertAndGetParticipant(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)

	_, err := client.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{
			UserId:          "u1",
			InstitutionName: "Bank A",
			BankCode:        "AAA",
			Role:            domain.RoleCommercialBank,
		},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := client.GetParticipantByUser(ctx, &compliancv1.GetParticipantByUserRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Found || got.Participant.InstitutionName != "Bank A" {
		t.Fatalf("unexpected participant: %+v", got)
	}
	// Default status applied.
	if got.Participant.Status != string(domain.StatusPending) {
		t.Errorf("status = %q, want PENDING", got.Participant.Status)
	}
}

func TestUpsertParticipant_MissingUserID(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	_, err := client.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "  "},
	})
	if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestGetParticipant_MissingArg(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	_, err := client.GetParticipantByUser(ctx, &compliancv1.GetParticipantByUserRequest{UserId: ""})
	if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestGetParticipant_NotFound(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	got, err := client.GetParticipantByUser(ctx, &compliancv1.GetParticipantByUserRequest{UserId: "ghost"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Found {
		t.Error("expected not found")
	}
}

func TestListParticipants_FilterAndBankCode(t *testing.T) {
	repo := repository.NewMemoryRepository()
	client, ctx := newTestClient(t, repo, nil)
	_, _ = client.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "u1", BankCode: "AAA", Status: "ACTIVE"},
	})
	_, _ = client.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "u2", BankCode: "BBB", Status: "ACTIVE"},
	})

	// bank_code: prefix search path.
	resp, err := client.ListParticipants(ctx, &compliancv1.ListParticipantsRequest{Search: "bank_code:AAA"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.Participants) != 1 || resp.Participants[0].UserId != "u1" {
		t.Fatalf("bank_code filter returned %+v", resp.Participants)
	}

	// status filter, non-bankcode search path.
	resp, err = client.ListParticipants(ctx, &compliancv1.ListParticipantsRequest{Status: "ACTIVE", Search: "anything"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.Participants) != 2 {
		t.Fatalf("status filter returned %d, want 2", len(resp.Participants))
	}
}

// --- Audit logs ---

func TestCreateAndGetAuditLogs(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	_, err := client.CreateAuditLog(ctx, &compliancv1.CreateAuditLogRequest{
		Entry: &compliancv1.AuditLogEntry{
			ActorSubject: "actor",
			ActionType:   "TEST",
			Category:     string(domain.CategorySession),
			Severity:     string(domain.SeverityInfo),
			Result:       "SUCCESS",
		},
	})
	if err != nil {
		t.Fatalf("create audit: %v", err)
	}

	resp, err := client.GetAuditLogs(ctx, &compliancv1.GetAuditLogsRequest{
		Category: string(domain.CategorySession),
		FromDate: time.Now().Add(-time.Hour).Format(time.RFC3339),
		ToDate:   time.Now().Add(time.Hour).Format(time.RFC3339),
		Page:     1,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("get audit: %v", err)
	}
	if len(resp.Logs) != 1 || resp.Logs[0].ActionType != "TEST" {
		t.Fatalf("unexpected logs: %+v", resp.Logs)
	}
}

// --- KYC / status governance ---

func seedParticipant(t *testing.T, client compliancv1.ComplianceServiceClient, ctx context.Context, userID, status string) {
	t.Helper()
	_, err := client.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: userID, Status: status, WalletAddress: "0x" + userID},
	})
	if err != nil {
		t.Fatalf("seed %s: %v", userID, err)
	}
}

func TestApproveKYC_Success(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	seedParticipant(t, client, ctx, "u1", string(domain.StatusCredentialRequested))

	resp, err := client.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "u1", ActorSubject: "cb"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if resp.Status != string(domain.StatusKYCApproved) || resp.PopNonce == "" {
		t.Fatalf("unexpected approve resp: %+v", resp)
	}
}

func TestApproveKYC_Errors(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)

	// missing subject
	if _, err := client.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
	// not found
	if _, err := client.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "ghost"}); codeOf(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
	// wrong precondition state
	seedParticipant(t, client, ctx, "u-active", string(domain.StatusActive))
	if _, err := client.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "u-active"}); codeOf(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", err)
	}
}

func TestManageParticipantStatus(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	seedParticipant(t, client, ctx, "u1", string(domain.StatusActive))

	// freeze
	resp, err := client.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: "u1", Status: string(domain.StatusFrozen), Reason: "risk",
	})
	if err != nil || resp.Status != string(domain.StatusFrozen) {
		t.Fatalf("freeze failed: resp=%+v err=%v", resp, err)
	}

	// revoke (critical severity branch)
	_, err = client.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: "u1", Status: string(domain.StatusRevoked), Reason: "fraud",
	})
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	// rejection path
	_, err = client.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: "u1", Status: "KYC_REJECTED", Reason: "bad docs",
	})
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}
}

func TestManageParticipantStatus_Errors(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	if _, err := client.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{Subject: "u1"}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
	if _, err := client.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{Subject: "ghost", Status: "ACTIVE"}); codeOf(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

// --- Circuit breaker / system params ---

func TestCircuitBreakerLifecycle(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	md := metadata.New(map[string]string{"x-actor-subject": "noc-1"})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// default unpaused
	st, err := client.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil || st.IsPaused {
		t.Fatalf("default status: %+v err=%v", st, err)
	}

	// pause (critical)
	tg, err := client.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "incident"})
	if err != nil || !tg.IsPaused {
		t.Fatalf("pause: %+v err=%v", tg, err)
	}
	st, _ = client.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if !st.IsPaused || st.UpdatedBy != "noc-1" {
		t.Fatalf("expected paused by noc-1, got %+v", st)
	}

	// resume
	tg, err = client.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"})
	if err != nil || tg.IsPaused {
		t.Fatalf("resume: %+v err=%v", tg, err)
	}
}

func TestToggleCircuitBreaker_MissingReason(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	if _, err := client.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: true}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestSystemParametersRoundTrip(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	_, err := client.UpdateSystemParameters(ctx, &compliancv1.UpdateSystemParametersRequest{
		TransactionMinimum: "10",
		TransactionMaximum: "1000",
		SlippageTolerance:  0.05,
		SettlementWindow:   3600,
		Reason:             "tuning",
		ActorSubject:       "gov",
	})
	if err != nil {
		t.Fatalf("update params: %v", err)
	}

	got, err := client.GetSystemParameters(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("get params: %v", err)
	}
	if got.TransactionMinimum != "10" || got.TransactionMaximum != "1000" {
		t.Errorf("tx min/max = %q/%q", got.TransactionMinimum, got.TransactionMaximum)
	}
	if got.SlippageTolerance != 0.05 || got.SettlementWindow != 3600 {
		t.Errorf("slippage/settlement = %v/%v", got.SlippageTolerance, got.SettlementWindow)
	}
}

func TestUpdateSystemParameters_MissingReason(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), nil)
	if _, err := client.UpdateSystemParameters(ctx, &compliancv1.UpdateSystemParametersRequest{}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

// --- PKI ---

func TestIssueParticipantCertificate(t *testing.T) {
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), testCA(t))
	resp, err := client.IssueParticipantCertificate(ctx, &compliancv1.IssueParticipantCertificateRequest{
		UserId:          "u1",
		Role:            domain.RoleCommercialBank,
		InstitutionName: "Bank A",
	})
	if err != nil {
		t.Fatalf("issue cert: %v", err)
	}
	if resp.CertPem == "" || resp.PrivKeyPem == "" || resp.ExpiresAt == "" {
		t.Fatalf("incomplete cert response: %+v", resp)
	}
}

func TestIssueParticipantCertificate_Errors(t *testing.T) {
	// missing args
	client, ctx := newTestClient(t, repository.NewMemoryRepository(), testCA(t))
	if _, err := client.IssueParticipantCertificate(ctx, &compliancv1.IssueParticipantCertificateRequest{UserId: "u1"}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}

	// CA not configured
	client2, ctx2 := newTestClient(t, repository.NewMemoryRepository(), nil)
	if _, err := client2.IssueParticipantCertificate(ctx2, &compliancv1.IssueParticipantCertificateRequest{
		UserId: "u1", Role: "r", InstitutionName: "Bank",
	}); codeOf(err) != codes.Unimplemented {
		t.Fatalf("want Unimplemented, got %v", err)
	}
}

func TestSignParticipantCSR(t *testing.T) {
	repo := repository.NewMemoryRepository()
	client, ctx := newTestClient(t, repo, testCA(t))

	// participant must exist with a wallet to exercise the registration branch.
	seedParticipant(t, client, ctx, "u1", string(domain.StatusCredentialRequested))

	csrPEM, _, err := pki.GenerateCSR("u1", "Bank A", domain.RoleCommercialBank, "BR")
	if err != nil {
		t.Fatalf("gen csr: %v", err)
	}

	resp, err := client.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{
		CsrPem: csrPEM,
		UserId: "u1",
		Role:   domain.RoleCommercialBank,
	})
	if err != nil {
		t.Fatalf("sign csr: %v", err)
	}
	if resp.CertPem == "" {
		t.Fatal("expected signed cert")
	}
}

func TestSignParticipantCSR_Errors(t *testing.T) {
	repo := repository.NewMemoryRepository()
	client, ctx := newTestClient(t, repo, testCA(t))

	// missing args
	if _, err := client.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{UserId: "u1"}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}

	csrPEM, _, _ := pki.GenerateCSR("u1", "Bank A", domain.RoleCommercialBank, "BR")

	// participant not found
	if _, err := client.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{
		CsrPem: csrPEM, UserId: "ghost", Role: domain.RoleCommercialBank,
	}); codeOf(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}

	// CA not configured
	client2, ctx2 := newTestClient(t, repository.NewMemoryRepository(), nil)
	if _, err := client2.SignParticipantCSR(ctx2, &compliancv1.SignParticipantCSRRequest{
		CsrPem: csrPEM, UserId: "u1", Role: "r",
	}); codeOf(err) != codes.Unimplemented {
		t.Fatalf("want Unimplemented, got %v", err)
	}
}

// --- context metadata helpers ---

func TestMetadataHelpers(t *testing.T) {
	md := metadata.New(map[string]string{
		"x-correlation-id": "corr-1",
		"x-forwarded-for":  "1.2.3.4",
		"x-actor-subject":  "actor-1",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	if correlationIDFromCtx(ctx) != "corr-1" {
		t.Error("correlation id")
	}
	if ipAddressFromCtx(ctx) != "1.2.3.4" {
		t.Error("ip address")
	}
	if actorFromCtx(ctx) != "actor-1" {
		t.Error("actor")
	}
	// empty context returns empty strings.
	empty := context.Background()
	if correlationIDFromCtx(empty) != "" || ipAddressFromCtx(empty) != "" || actorFromCtx(empty) != "" {
		t.Error("expected empty metadata extraction")
	}
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "true" || boolStr(false) != "false" {
		t.Fatal("boolStr mismatch")
	}
}
