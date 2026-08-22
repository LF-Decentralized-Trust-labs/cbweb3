// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ── test doubles ──────────────────────────────────────────────────────────────

// recordingRegistry captures RegisterParticipant calls and lets tests force an error.
type recordingRegistry struct {
	registry.NoopRegistryClient
	mu           sync.Mutex
	calls        int
	verifyCalls  int
	verifiedAddr string
	lastAddr     string
	lastInst     string
	lastRole     string
	returnErr    error
}

func (r *recordingRegistry) RegisterParticipant(_ context.Context, addr, inst, role string, _ [32]byte) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.lastAddr, r.lastInst, r.lastRole = addr, inst, role
	if r.returnErr != nil {
		return "", r.returnErr
	}
	return "0xtx", nil
}

// VerifyParticipant records the second step of the two-step onboarding (R1-10.6).
func (r *recordingRegistry) VerifyParticipant(_ context.Context, addr string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verifyCalls++
	r.verifiedAddr = addr
	if r.returnErr != nil {
		return "", r.returnErr
	}
	return "0xverify", nil
}

func (r *recordingRegistry) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recordingRegistry) verifyCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.verifyCalls
}

// errRepo wraps a Repository and forces selected methods to error.
type errRepo struct {
	repository.Repository
	failGet      bool
	failUpsert   bool
	failAudit    bool
	failGetAudit bool
}

func (e *errRepo) GetParticipantByUser(ctx context.Context, id string) (repository.Participant, bool, error) {
	if e.failGet {
		return repository.Participant{}, false, context.DeadlineExceeded
	}
	return e.Repository.GetParticipantByUser(ctx, id)
}

func (e *errRepo) UpsertParticipant(ctx context.Context, p repository.Participant) error {
	if e.failUpsert {
		return context.DeadlineExceeded
	}
	return e.Repository.UpsertParticipant(ctx, p)
}

func (e *errRepo) GetAuditLogs(ctx context.Context, f repository.AuditFilter) ([]repository.AuditRecord, error) {
	if e.failGetAudit {
		return nil, context.DeadlineExceeded
	}
	return e.Repository.GetAuditLogs(ctx, f)
}

func newCATestService(t *testing.T) (*complianceService, *recordingRegistry) {
	t.Helper()
	certPEM, keyPEM, err := pki.GenerateSelfSignedCA("Central Bank A CA", "CB-A", 5)
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	ca := compliancepki.NewCAFromPEM(certPEM, keyPEM)
	reg := &recordingRegistry{}
	return &complianceService{repo: repository.NewMemoryRepository(), ca: ca, blockchain: reg}, reg
}

// ── New constructor ───────────────────────────────────────────────────────────

func TestNew_NilBlockchainUsesNoop(t *testing.T) {
	t.Parallel()
	srv, err := New(repository.NewMemoryRepository(), nil, nil, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil grpc server")
	}
}

// ── Participant handlers ──────────────────────────────────────────────────────

func TestUpsertParticipant_Success_DefaultsStatus(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	expiry := timestamppb.New(time.Now().Add(24 * time.Hour))
	resp, err := svc.UpsertParticipant(context.Background(), &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{
			UserId:            "bank-user-1",
			InstitutionName:   "Bank A",
			CertificateExpiry: expiry,
			PopNonceExpiresAt: expiry,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success")
	}
	got, found, _ := svc.repo.GetParticipantByUser(context.Background(), "bank-user-1")
	if !found {
		t.Fatal("participant not stored")
	}
	if got.Status != string(domain.StatusPending) {
		t.Errorf("status = %q, want default PENDING", got.Status)
	}
	if got.CertificateExpiry == nil || got.PopNonceExpiresAt == nil {
		t.Error("expected expiry timestamps to be stored")
	}
}

func TestUpsertParticipant_MissingUserID(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	_, err := svc.UpsertParticipant(context.Background(), &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "  "},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestUpsertParticipant_RepoError(t *testing.T) {
	t.Parallel()
	svc := &complianceService{repo: &errRepo{Repository: repository.NewMemoryRepository(), failUpsert: true}}
	_, err := svc.UpsertParticipant(context.Background(), &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "x"},
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestGetParticipantByUser(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	expiry := timestamppb.New(time.Now().Add(time.Hour))
	_, _ = svc.UpsertParticipant(context.Background(), &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "u1", InstitutionName: "Bank", CertificateExpiry: expiry, PopNonceExpiresAt: expiry},
	})

	resp, err := svc.GetParticipantByUser(context.Background(), &compliancv1.GetParticipantByUserRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Found || resp.Participant.UserId != "u1" {
		t.Errorf("expected found participant u1, got %+v", resp)
	}
	if resp.Participant.CertificateExpiry == nil || resp.Participant.PopNonceExpiresAt == nil {
		t.Error("expected proto expiry timestamps")
	}

	// Not found.
	resp2, _ := svc.GetParticipantByUser(context.Background(), &compliancv1.GetParticipantByUserRequest{UserId: "missing"})
	if resp2.Found {
		t.Error("expected not found")
	}

	// Missing user id.
	_, err = svc.GetParticipantByUser(context.Background(), &compliancv1.GetParticipantByUserRequest{UserId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestGetParticipantByUser_RepoError(t *testing.T) {
	t.Parallel()
	svc := &complianceService{repo: &errRepo{Repository: repository.NewMemoryRepository(), failGet: true}}
	_, err := svc.GetParticipantByUser(context.Background(), &compliancv1.GetParticipantByUserRequest{UserId: "u"})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestListParticipants_StatusAndBankCode(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{Participant: &compliancv1.Participant{UserId: "a", BankCode: "BANK-A", Status: "ACTIVE"}})
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{Participant: &compliancv1.Participant{UserId: "b", BankCode: "BANK-B", Status: "ACTIVE"}})

	// bank_code: prefix filter.
	resp, err := svc.ListParticipants(ctx, &compliancv1.ListParticipantsRequest{Search: "bank_code:BANK-A"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Participants) != 1 || resp.Participants[0].UserId != "a" {
		t.Errorf("expected only bank A participant, got %+v", resp.Participants)
	}

	// Status filter (plain search path).
	resp2, _ := svc.ListParticipants(ctx, &compliancv1.ListParticipantsRequest{Status: "ACTIVE"})
	if len(resp2.Participants) != 2 {
		t.Errorf("expected 2 active, got %d", len(resp2.Participants))
	}
}

// ── Audit log handlers ────────────────────────────────────────────────────────

func TestCreateAndGetAuditLogs(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()
	_, err := svc.CreateAuditLog(ctx, &compliancv1.CreateAuditLogRequest{
		Entry: &compliancv1.AuditLogEntry{
			ActorSubject: "actor", ActionType: "TEST", Category: "SESSION", Severity: "INFO", Result: "SUCCESS",
		},
	})
	if err != nil {
		t.Fatalf("create audit: %v", err)
	}

	resp, err := svc.GetAuditLogs(ctx, &compliancv1.GetAuditLogsRequest{
		Category: "SESSION", FromDate: time.Now().Add(-time.Hour).Format(time.RFC3339), ToDate: time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("get audit: %v", err)
	}
	if len(resp.Logs) != 1 || resp.Logs[0].ActionType != "TEST" {
		t.Errorf("expected 1 audit record TEST, got %+v", resp.Logs)
	}
	if resp.Logs[0].Timestamp == "" {
		t.Error("expected formatted timestamp")
	}
}

func TestGetAuditLogs_InvalidDatesIgnored(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	_, _ = svc.CreateAuditLog(context.Background(), &compliancv1.CreateAuditLogRequest{Entry: &compliancv1.AuditLogEntry{ActionType: "X"}})
	resp, err := svc.GetAuditLogs(context.Background(), &compliancv1.GetAuditLogsRequest{FromDate: "not-a-date", ToDate: "also-bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Logs) != 1 {
		t.Errorf("expected 1 record, got %d", len(resp.Logs))
	}
}

func TestGetAuditLogs_RepoError(t *testing.T) {
	t.Parallel()
	svc := &complianceService{repo: &errRepo{Repository: repository.NewMemoryRepository(), failGetAudit: true}}
	_, err := svc.GetAuditLogs(context.Background(), &compliancv1.GetAuditLogsRequest{})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ── PKI handlers ──────────────────────────────────────────────────────────────

func TestIssueParticipantCertificate_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newCATestService(t)
	resp, err := svc.IssueParticipantCertificate(context.Background(), &compliancv1.IssueParticipantCertificateRequest{
		UserId: "bank-user", Role: "BANK", InstitutionName: "Bank A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CertPem == "" || resp.PrivKeyPem == "" || resp.ExpiresAt == "" {
		t.Errorf("expected populated cert response, got %+v", resp)
	}
}

func TestIssueParticipantCertificate_MissingFields(t *testing.T) {
	t.Parallel()
	svc, _ := newCATestService(t)
	_, err := svc.IssueParticipantCertificate(context.Background(), &compliancv1.IssueParticipantCertificateRequest{UserId: "x"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestIssueParticipantCertificate_NoCA(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	_, err := svc.IssueParticipantCertificate(context.Background(), &compliancv1.IssueParticipantCertificateRequest{
		UserId: "x", Role: "BANK", InstitutionName: "Bank",
	})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented, got %v", err)
	}
}

func TestSignParticipantCSR_Success_NoChainCall(t *testing.T) {
	t.Parallel()
	svc, reg := newCATestService(t)
	ctx := context.Background()

	// Onboard participant first, with wallet set.
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "bank-user", InstitutionName: "Bank A", WalletAddress: "0xabc", Status: "PENDING"},
	})

	csrPEM, _, err := pki.GenerateCSR("bank-user", "Bank A", "BANK", "BR")
	if err != nil {
		t.Fatalf("generate CSR: %v", err)
	}

	resp, err := svc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{
		CsrPem: csrPEM, UserId: "bank-user", Role: "BANK",
	})
	if err != nil {
		t.Fatalf("sign CSR: %v", err)
	}
	if resp.CertPem == "" {
		t.Error("expected signed cert")
	}
	// On-chain registration moved to ApproveKYC; CSR signing must not touch the chain.
	if reg.callCount() != 0 {
		t.Errorf("expected no on-chain registration in CSR signing, got %d", reg.callCount())
	}
}

func TestSignParticipantCSR_Validation(t *testing.T) {
	t.Parallel()
	svc, _ := newCATestService(t)
	ctx := context.Background()

	// Missing fields.
	if _, err := svc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	// No CA.
	noCA := newTestService()
	if _, err := noCA.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{CsrPem: "x", UserId: "u", Role: "BANK"}); status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented, got %v", err)
	}

	// Invalid CSR.
	if _, err := svc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{CsrPem: "garbage", UserId: "u", Role: "BANK"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for bad CSR, got %v", err)
	}

	// Participant not found.
	csrPEM, _, _ := pki.GenerateCSR("ghost", "Bank", "BANK", "BR")
	if _, err := svc.SignParticipantCSR(ctx, &compliancv1.SignParticipantCSRRequest{CsrPem: csrPEM, UserId: "ghost", Role: "BANK"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// ── Governance: ApproveKYC ────────────────────────────────────────────────────

func TestApproveKYC_Success(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "bank-user", Status: string(domain.StatusCredentialRequested)},
	})
	resp, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "bank-user", ActorSubject: "cb", Reason: "ok"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if resp.Status != string(domain.StatusKYCApproved) || resp.PopNonce == "" {
		t.Errorf("expected KYC_APPROVED with nonce, got %+v", resp)
	}
	got, _, _ := svc.repo.GetParticipantByUser(ctx, "bank-user")
	if got.PopNonce != resp.PopNonce || got.PopNonceExpiresAt == nil {
		t.Error("expected pop nonce persisted with expiry")
	}
}

func TestApproveKYC_Validation(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()

	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: ""}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "missing"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Wrong status.
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "active-user", Status: string(domain.StatusActive)},
	})
	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "active-user"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestApproveKYC_RegistersOnChain(t *testing.T) {
	t.Parallel()
	svc, reg := newCATestService(t)
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{
			UserId: "bank-user", InstitutionName: "Bank A", WalletAddress: "0xabc",
			Role: "commercial_bank", Status: string(domain.StatusCredentialRequested),
		},
	})

	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "bank-user", ActorSubject: "cb", Reason: "ok"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if reg.callCount() != 1 {
		t.Fatalf("expected on-chain registration at approval, got %d", reg.callCount())
	}
	// Two-step onboarding (R1-10.6): approval must ALSO verify, or the bank stays Pending
	// and HTLC.lock reverts ParticipantNotVerified.
	if reg.verifyCount() != 1 || reg.verifiedAddr != "0xabc" {
		t.Fatalf("expected verifyParticipant(0xabc) at approval, got count=%d addr=%q", reg.verifyCount(), reg.verifiedAddr)
	}
	if reg.lastAddr != "0xabc" || reg.lastRole != "commercial_bank" || reg.lastInst != "Bank A" {
		t.Errorf("registered wrong participant: addr=%q role=%q inst=%q", reg.lastAddr, reg.lastRole, reg.lastInst)
	}
}

func TestApproveKYC_NoWallet_SkipsChain(t *testing.T) {
	t.Parallel()
	svc, reg := newCATestService(t)
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "bank-user", Status: string(domain.StatusCredentialRequested)},
	})
	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "bank-user"}); err != nil {
		t.Fatalf("approve should succeed without wallet, got %v", err)
	}
	if reg.callCount() != 0 {
		t.Errorf("expected no chain call without wallet, got %d", reg.callCount())
	}
}

func TestApproveKYC_ChainErrorFatal(t *testing.T) {
	t.Parallel()
	svc, reg := newCATestService(t)
	reg.returnErr = context.DeadlineExceeded
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{
			UserId: "bank-user", InstitutionName: "Bank A", WalletAddress: "0xabc",
			Role: "commercial_bank", Status: string(domain.StatusCredentialRequested),
		},
	})
	if _, err := svc.ApproveKYC(ctx, &compliancv1.ApproveKYCRequest{Subject: "bank-user"}); status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal on chain error, got %v", err)
	}
	// A failed registration must not mark the participant KYC_APPROVED.
	got, _, _ := svc.repo.GetParticipantByUser(ctx, "bank-user")
	if got.Status == string(domain.StatusKYCApproved) {
		t.Error("participant should not be KYC_APPROVED after failed on-chain registration")
	}
}

// ── ManageParticipantStatus ───────────────────────────────────────────────────

func TestManageParticipantStatus(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()
	_, _ = svc.UpsertParticipant(ctx, &compliancv1.UpsertParticipantRequest{
		Participant: &compliancv1.Participant{UserId: "bank-user", Status: string(domain.StatusActive)},
	})

	resp, err := svc.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{
		Subject: "bank-user", Status: string(domain.StatusRevoked), Reason: "fraud",
	})
	if err != nil {
		t.Fatalf("manage: %v", err)
	}
	if resp.Status != string(domain.StatusRevoked) {
		t.Errorf("status = %q", resp.Status)
	}

	// Validation paths.
	if _, err := svc.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{Subject: "", Status: "X"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	if _, err := svc.ManageParticipantStatus(ctx, &compliancv1.ManageParticipantStatusRequest{Subject: "ghost", Status: "ACTIVE"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

// ── Circuit breaker ───────────────────────────────────────────────────────────

func TestCircuitBreaker_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "noc-operator", Method: "mtls"})

	// Default state: not paused.
	st, err := svc.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.IsPaused {
		t.Error("expected not paused by default")
	}

	// Pause.
	tog, err := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "incident"})
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !tog.IsPaused {
		t.Error("expected paused")
	}

	st2, _ := svc.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if !st2.IsPaused || st2.UpdatedBy != "noc-operator" {
		t.Errorf("expected paused state with updater, got %+v", st2)
	}

	// Resume.
	tog2, _ := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"})
	if tog2.IsPaused {
		t.Error("expected resumed")
	}
}

func TestToggleCircuitBreaker_MissingReason(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	_, err := svc.ToggleCircuitBreaker(context.Background(), &compliancv1.ToggleCircuitBreakerRequest{Pause: true})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// ── System parameters ─────────────────────────────────────────────────────────

func TestSystemParameters_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.UpdateSystemParameters(ctx, &compliancv1.UpdateSystemParametersRequest{
		TransactionMinimum: "10", TransactionMaximum: "1000",
		SlippageTolerance: 0.5, SettlementWindow: 3600,
		Reason: "tuning", ActorSubject: "treasury",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := svc.GetSystemParameters(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TransactionMinimum != "10" || got.TransactionMaximum != "1000" {
		t.Errorf("tx min/max mismatch: %+v", got)
	}
	if got.SlippageTolerance != 0.5 || got.SettlementWindow != 3600 {
		t.Errorf("slippage/settlement mismatch: %+v", got)
	}
}

func TestUpdateSystemParameters_MissingReason(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	_, err := svc.UpdateSystemParameters(context.Background(), &compliancv1.UpdateSystemParametersRequest{Reason: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestUpdateSystemParameters_ActorFromCtx(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "ctx-actor", Method: "mtls"})
	_, err := svc.UpdateSystemParameters(ctx, &compliancv1.UpdateSystemParametersRequest{Reason: "r"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}

// ── ctx helpers ───────────────────────────────────────────────────────────────

func TestCtxHelpers(t *testing.T) {
	t.Parallel()
	md := metadata.Pairs(
		"x-correlation-id", "corr-1",
		"x-forwarded-for", "1.2.3.4",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	// The actor comes from the authenticated identity, not from metadata: the
	// x-actor-subject header this test used to set is no longer read at all.
	ctx = authz.NewContext(ctx, &authz.Identity{Subject: "actor-1", Method: "mtls"})
	if correlationIDFromCtx(ctx) != "corr-1" {
		t.Error("correlation id")
	}
	if ipAddressFromCtx(ctx) != "1.2.3.4" {
		t.Error("ip address")
	}
	if actorFromCtx(ctx) != "actor-1" {
		t.Error("actor")
	}

	// No metadata → empty.
	if correlationIDFromCtx(context.Background()) != "" || ipAddressFromCtx(context.Background()) != "" || actorFromCtx(context.Background()) != "" {
		t.Error("expected empty values without metadata")
	}
}

func TestBoolStr(t *testing.T) {
	t.Parallel()
	if boolStr(true) != "true" || boolStr(false) != "false" {
		t.Error("boolStr mismatch")
	}
}
