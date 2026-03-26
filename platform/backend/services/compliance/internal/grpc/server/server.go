package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type complianceService struct {
	compliancv1.UnimplementedComplianceServiceServer
	repo       repository.Repository
	ca         *compliancepki.CA
	blockchain registry.RegistryWriter
}

// New builds a configured gRPC server with all compliance handlers.
// bc may be nil; when nil, a NoopRegistryClient is used (dev/test mode).
func New(repo repository.Repository, ca *compliancepki.CA, bc registry.RegistryWriter) *grpc.Server {
	if bc == nil {
		bc = registry.NoopRegistryClient{}
	}
	svc := &complianceService{repo: repo, ca: ca, blockchain: bc}
	grpcServer := grpc.NewServer()
	compliancv1.RegisterComplianceServiceServer(grpcServer, svc)
	return grpcServer
}

// --- Participant ---

func (s *complianceService) UpsertParticipant(ctx context.Context, req *compliancv1.UpsertParticipantRequest) (*compliancv1.UpsertParticipantResponse, error) {
	if strings.TrimSpace(req.Participant.UserId) == "" {
		return nil, status.Error(codes.InvalidArgument, "participant.user_id is required")
	}
	var certExpiry *time.Time
	if req.Participant.CertificateExpiry != nil {
		t := req.Participant.CertificateExpiry.AsTime()
		certExpiry = &t
	}
	var popExpiry *time.Time
	if req.Participant.PopNonceExpiresAt != nil {
		t := req.Participant.PopNonceExpiresAt.AsTime()
		popExpiry = &t
	}
	p := repository.Participant{
		UserID:              req.Participant.UserId,
		InstitutionName:     req.Participant.InstitutionName,
		CNPJ:                req.Participant.Cnpj,
		BankCode:            req.Participant.BankCode,
		CountryCode:         req.Participant.CountryCode,
		Role:                req.Participant.Role,
		WalletAddress:       req.Participant.WalletAddress,
		Status:              req.Participant.Status,
		CertificateData:     req.Participant.CertificateData,
		CertificateExpiry:   certExpiry,
		BlockchainPubKeyHex: req.Participant.BlockchainPubKeyHex,
		CsrPem:              req.Participant.CsrPem,
		PopNonce:            req.Participant.PopNonce,
		PopNonceExpiresAt:   popExpiry,
	}
	if p.Status == "" {
		p.Status = string(domain.StatusPending)
	}
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &compliancv1.UpsertParticipantResponse{Success: true}, nil
}

func (s *complianceService) GetParticipantByUser(ctx context.Context, req *compliancv1.GetParticipantByUserRequest) (*compliancv1.GetParticipantByUserResponse, error) {
	if strings.TrimSpace(req.UserId) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	p, found, err := s.repo.GetParticipantByUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &compliancv1.GetParticipantByUserResponse{Found: false}, nil
	}
	return &compliancv1.GetParticipantByUserResponse{
		Found:       true,
		Participant: participantToProto(p),
	}, nil
}

func (s *complianceService) ListParticipants(ctx context.Context, req *compliancv1.ListParticipantsRequest) (*compliancv1.ListParticipantsResponse, error) {
	list, err := s.repo.ListParticipants(ctx, repository.ParticipantFilter{
		Status: req.Status,
		Search: req.Search,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	result := make([]*compliancv1.Participant, len(list))
	for i, p := range list {
		result[i] = participantToProto(p)
	}
	return &compliancv1.ListParticipantsResponse{Participants: result}, nil
}

// --- Audit Log ---

func (s *complianceService) CreateAuditLog(ctx context.Context, req *compliancv1.CreateAuditLogRequest) (*compliancv1.CreateAuditLogResponse, error) {
	entry := repository.AuditEntry{
		ActorSubject:  req.Entry.ActorSubject,
		ActorAddress:  req.Entry.ActorAddress,
		ActionType:    req.Entry.ActionType,
		TargetSubject: req.Entry.TargetSubject,
		CorrelationID: req.Entry.CorrelationId,
		IPAddress:     req.Entry.IpAddress,
		Result:        req.Entry.Result,
		Category:      req.Entry.Category,
		Severity:      req.Entry.Severity,
		Details:       req.Entry.Details,
	}
	if err := s.repo.CreateAuditLog(ctx, entry); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &compliancv1.CreateAuditLogResponse{Success: true}, nil
}

func (s *complianceService) GetAuditLogs(ctx context.Context, req *compliancv1.GetAuditLogsRequest) (*compliancv1.GetAuditLogsResponse, error) {
	f := repository.AuditFilter{
		Category: req.Category,
		Severity: req.Severity,
		Page:     int(req.Page),
		Limit:    int(req.Limit),
	}
	if req.FromDate != "" {
		t, err := time.Parse(time.RFC3339, req.FromDate)
		if err == nil {
			f.FromDate = t
		}
	}
	if req.ToDate != "" {
		t, err := time.Parse(time.RFC3339, req.ToDate)
		if err == nil {
			f.ToDate = t
		}
	}

	records, err := s.repo.GetAuditLogs(ctx, f)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	logs := make([]*compliancv1.AuditLogRecord, len(records))
	for i, r := range records {
		logs[i] = &compliancv1.AuditLogRecord{
			LogId:         r.LogID,
			Timestamp:     r.Timestamp.UTC().Format(time.RFC3339),
			ActorSubject:  r.ActorSubject,
			ActorAddress:  r.ActorAddress,
			ActionType:    r.ActionType,
			TargetSubject: r.TargetSubject,
			CorrelationId: r.CorrelationID,
			Category:      r.Category,
			Severity:      r.Severity,
			Result:        r.Result,
			Details:       r.Details,
		}
	}
	return &compliancv1.GetAuditLogsResponse{Logs: logs}, nil
}

// --- PKI ---

func (s *complianceService) IssueParticipantCertificate(ctx context.Context, req *compliancv1.IssueParticipantCertificateRequest) (*compliancv1.IssueParticipantCertificateResponse, error) {
	if req.UserId == "" || req.Role == "" || req.InstitutionName == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, role, and institution_name are required")
	}
	if s.ca == nil {
		return nil, status.Error(codes.Unimplemented, "CA not configured")
	}

	issued, err := s.ca.IssueParticipantCert(req.UserId, req.InstitutionName, req.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "issue certificate: %v", err)
	}

	expiresAt := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)

	s.emitAudit(ctx, "ISSUE_CERTIFICATE", req.UserId, "", req.UserId,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo),
		fmt.Sprintf(`{"role":%q,"institution":%q}`, req.Role, req.InstitutionName))

	return &compliancv1.IssueParticipantCertificateResponse{
		CertPem:    issued.CertPEM,
		PrivKeyPem: issued.PrivKeyPEM,
		ExpiresAt:  expiresAt,
	}, nil
}

// SignParticipantCSR signs a PKCS#10 CSR submitted by a participant, updates
// only the certificate fields in the participant record, and (best-effort)
// registers on the blockchain using the wallet address set during onboarding.
//
// The participant's wallet address is set by the KMS during OnboardParticipant
// and must be preserved here — it is NOT derived from CB_PRIVATE_KEY.
func (s *complianceService) SignParticipantCSR(ctx context.Context, req *compliancv1.SignParticipantCSRRequest) (*compliancv1.SignParticipantCSRResponse, error) {
	if req.CsrPem == "" || req.UserId == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "csr_pem, user_id, and role are required")
	}
	if s.ca == nil {
		return nil, status.Error(codes.Unimplemented, "CA not configured")
	}

	issued, err := s.ca.SignCSR(req.CsrPem)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sign CSR: %v", err)
	}

	// Fetch existing record to preserve the KMS-generated wallet address and
	// other fields set during onboarding that are not present in the CSR request.
	existing, found, fetchErr := s.repo.GetParticipantByUser(ctx, req.UserId)
	if fetchErr != nil {
		return nil, status.Errorf(codes.Internal, "sign CSR: lookup participant: %v", fetchErr)
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "participant %q not found; call POST /compliance/register first", req.UserId)
	}
	if existing.WalletAddress == "" {
		log.Printf("WARN: SignParticipantCSR: participant %s has no wallet address — ApproveKYC will fail until wallet is set", req.UserId)
	}

	expiresAt := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)

	institutionName := req.InstitutionName
	if institutionName == "" {
		institutionName = existing.InstitutionName
	}
	cnpj := req.Cnpj
	if cnpj == "" {
		cnpj = existing.CNPJ
	}

	p := repository.Participant{
		UserID:          req.UserId,
		InstitutionName: institutionName,
		CNPJ:            cnpj,
		BankCode:        existing.BankCode,
		CountryCode:     existing.CountryCode,
		Role:            req.Role,
		Status:          existing.Status,
		WalletAddress:   existing.WalletAddress,
		CertificateData: issued.CertPEM,
	}
	if t, err2 := time.Parse(time.RFC3339, expiresAt); err2 == nil {
		p.CertificateExpiry = &t
	}
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert participant: %v", err)
	}

	if existing.WalletAddress != "" {
		if _, err := s.blockchain.RegisterParticipant(ctx, existing.WalletAddress, institutionName, req.Role, [32]byte{}); err != nil {
			log.Printf("WARN: SignParticipantCSR: on-chain registration failed (non-fatal): %v", err)
		}
	}

	s.emitAudit(ctx, "SIGN_CSR", actorFromCtx(ctx), "", req.UserId,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo),
		fmt.Sprintf(`{"role":%q,"institution":%q}`, req.Role, req.InstitutionName))

	return &compliancv1.SignParticipantCSRResponse{
		CertPem:   issued.CertPEM,
		ExpiresAt: expiresAt,
	}, nil
}

// --- Governance ---

// popNonceTTL is the time-to-live for Proof of Possession nonces (72 hours).
// The commercial bank discovers the nonce via polling and must complete
// onboarding within this window.
const popNonceTTL = 72 * time.Hour

// ApproveKYC sets the participant status to KYC_APPROVED and generates a PoP
// nonce for the commercial bank to sign with its secp256k1 key.
//
// Unlike the legacy flow, ApproveKYC no longer activates the participant
// on-chain or sets status to ACTIVE. On-chain registration happens in
// CompleteOnboarding after the bank proves wallet ownership.
func (s *complianceService) ApproveKYC(ctx context.Context, req *compliancv1.ApproveKYCRequest) (*compliancv1.ApproveKYCResponse, error) {
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	p, found, err := s.repo.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "participant %q not found", req.Subject)
	}

	if p.Status != string(domain.StatusCredentialRequested) && p.Status != string(domain.StatusPending) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"participant %q has status %q; expected CREDENTIAL_REQUESTED or PENDING", req.Subject, p.Status)
	}

	// Generate 32-byte PoP nonce for the commercial bank to sign with secp256k1.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, status.Errorf(codes.Internal, "generate pop nonce: %v", err)
	}
	popNonce := hex.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(popNonceTTL)

	p.Status = string(domain.StatusKYCApproved)
	p.PopNonce = popNonce
	p.PopNonceExpiresAt = &expiresAt

	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	detailsJSON, _ := json.Marshal(map[string]string{
		"status": string(domain.StatusKYCApproved),
		"reason": req.Reason,
	})
	s.emitAudit(ctx, "APPROVE_KYC", req.ActorSubject, "", req.Subject,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo), string(detailsJSON))

	return &compliancv1.ApproveKYCResponse{
		Subject:  req.Subject,
		Status:   string(domain.StatusKYCApproved),
		PopNonce: popNonce,
	}, nil
}

func (s *complianceService) ManageParticipantStatus(ctx context.Context, req *compliancv1.ManageParticipantStatusRequest) (*compliancv1.ManageParticipantStatusResponse, error) {
	if req.Subject == "" || req.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and status are required")
	}

	p, found, err := s.repo.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "participant %q not found", req.Subject)
	}

	p.Status = req.Status
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sev := string(domain.SeverityWarning)
	if req.Status == string(domain.StatusRevoked) {
		sev = string(domain.SeverityCritical)
	}
	detailsJSON, _ := json.Marshal(map[string]string{"status": req.Status, "reason": req.Reason})
	s.emitAudit(ctx, "MANAGE_PARTICIPANT_STATUS", "", "", req.Subject,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryFreeze), sev, string(detailsJSON))

	return &compliancv1.ManageParticipantStatusResponse{Subject: req.Subject, Status: req.Status}, nil
}

const (
	paramCircuitBreakerPaused    = "circuit_breaker_paused"
	paramCircuitBreakerUpdatedBy = "circuit_breaker_updated_by"
	paramCircuitBreakerUpdatedAt = "circuit_breaker_updated_at"
	paramTxMinimum               = "tx_minimum"
	paramTxMaximum               = "tx_maximum"
	paramSlippage                = "slippage_tolerance"
	paramSettlementWindow        = "settlement_window"
)

func (s *complianceService) GetCircuitBreakerStatus(ctx context.Context, _ *emptypb.Empty) (*compliancv1.GetCircuitBreakerStatusResponse, error) {
	paused, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerPaused)
	updatedBy, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedBy)
	updatedAt, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedAt)

	return &compliancv1.GetCircuitBreakerStatusResponse{
		IsPaused:   paused == "true",
		LastUpdate: updatedAt,
		UpdatedBy:  updatedBy,
	}, nil
}

func (s *complianceService) ToggleCircuitBreaker(ctx context.Context, req *compliancv1.ToggleCircuitBreakerRequest) (*compliancv1.ToggleCircuitBreakerResponse, error) {
	if req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	actorSubject := actorFromCtx(ctx)

	_ = s.repo.UpsertSystemParameter(ctx, repository.SystemParameter{Key: paramCircuitBreakerPaused, Value: boolStr(req.Pause), UpdatedBy: actorSubject})
	_ = s.repo.UpsertSystemParameter(ctx, repository.SystemParameter{Key: paramCircuitBreakerUpdatedBy, Value: actorSubject, UpdatedBy: actorSubject})
	_ = s.repo.UpsertSystemParameter(ctx, repository.SystemParameter{Key: paramCircuitBreakerUpdatedAt, Value: now, UpdatedBy: actorSubject})

	sev := string(domain.SeverityWarning)
	if req.Pause {
		sev = string(domain.SeverityCritical)
	}
	detailsJSON, _ := json.Marshal(map[string]interface{}{"pause": req.Pause, "reason": req.Reason})
	s.emitAudit(ctx, "TOGGLE_CIRCUIT_BREAKER", actorSubject, "", "",
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCircuitBreaker), sev, string(detailsJSON))

	return &compliancv1.ToggleCircuitBreakerResponse{IsPaused: req.Pause}, nil
}

func (s *complianceService) GetSystemParameters(ctx context.Context, _ *emptypb.Empty) (*compliancv1.GetSystemParametersResponse, error) {
	txMin, _, _ := s.repo.GetSystemParameter(ctx, paramTxMinimum)
	txMax, _, _ := s.repo.GetSystemParameter(ctx, paramTxMaximum)
	slippageStr, _, _ := s.repo.GetSystemParameter(ctx, paramSlippage)
	settlementStr, _, _ := s.repo.GetSystemParameter(ctx, paramSettlementWindow)

	slippage, _ := strconv.ParseFloat(slippageStr, 64)
	settlement, _ := strconv.ParseInt(settlementStr, 10, 64)

	return &compliancv1.GetSystemParametersResponse{
		TransactionMinimum: txMin,
		TransactionMaximum: txMax,
		SlippageTolerance:  slippage,
		SettlementWindow:   settlement,
	}, nil
}

func (s *complianceService) UpdateSystemParameters(ctx context.Context, req *compliancv1.UpdateSystemParametersRequest) (*compliancv1.UpdateSystemParametersResponse, error) {
	if req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}
	actor := req.ActorSubject
	if actor == "" {
		actor = actorFromCtx(ctx)
	}

	params := []repository.SystemParameter{
		{Key: paramTxMinimum, Value: req.TransactionMinimum, UpdatedBy: actor},
		{Key: paramTxMaximum, Value: req.TransactionMaximum, UpdatedBy: actor},
		{Key: paramSlippage, Value: strconv.FormatFloat(req.SlippageTolerance, 'f', -1, 64), UpdatedBy: actor},
		{Key: paramSettlementWindow, Value: strconv.FormatInt(req.SettlementWindow, 10), UpdatedBy: actor},
	}
	for _, p := range params {
		if err := s.repo.UpsertSystemParameter(ctx, p); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"tx_min": req.TransactionMinimum, "tx_max": req.TransactionMaximum,
		"slippage": req.SlippageTolerance, "settlement": req.SettlementWindow,
		"reason": req.Reason,
	})
	s.emitAudit(ctx, "UPDATE_SYSTEM_PARAMETERS", actor, "", "",
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryParameter), string(domain.SeverityWarning), string(detailsJSON))

	return &compliancv1.UpdateSystemParametersResponse{Success: true}, nil
}

// --- Audit helper ---

func (s *complianceService) emitAudit(ctx context.Context, action, actor, actorAddr, target, corrID, ip, result, category, severity, details string) {
	go func() {
		if err := s.repo.CreateAuditLog(context.Background(), repository.AuditEntry{
			ActionType:    action,
			ActorSubject:  actor,
			ActorAddress:  actorAddr,
			TargetSubject: target,
			CorrelationID: corrID,
			IPAddress:     ip,
			Result:        result,
			Category:      category,
			Severity:      severity,
			Details:       details,
		}); err != nil {
			log.Printf("WARN: audit emit failed: %v", err)
		}
	}()
}

func correlationIDFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func ipAddressFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func actorFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-actor-subject"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func participantToProto(p repository.Participant) *compliancv1.Participant {
	result := &compliancv1.Participant{
		UserId:              p.UserID,
		InstitutionName:     p.InstitutionName,
		Cnpj:                p.CNPJ,
		BankCode:            p.BankCode,
		CountryCode:         p.CountryCode,
		Role:                p.Role,
		WalletAddress:       p.WalletAddress,
		Status:              p.Status,
		CertificateData:     p.CertificateData,
		BlockchainPubKeyHex: p.BlockchainPubKeyHex,
		CsrPem:              p.CsrPem,
		PopNonce:            p.PopNonce,
	}
	if p.CertificateExpiry != nil {
		result.CertificateExpiry = timestamppb.New(*p.CertificateExpiry)
	}
	if p.PopNonceExpiresAt != nil {
		result.PopNonceExpiresAt = timestamppb.New(*p.PopNonceExpiresAt)
	}
	return result
}
