package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/grpc/jsoncodec"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type complianceService struct {
	repo       repository.Repository
	ca         *compliancepki.CA
	blockchain registry.RegistryWriter
}

type complianceServiceServer interface {
	UpsertParticipant(ctx context.Context, req *contract.UpsertParticipantRequest) (*contract.UpsertParticipantResponse, error)
	GetParticipantByUser(ctx context.Context, req *contract.GetParticipantByUserRequest) (*contract.GetParticipantByUserResponse, error)
	ListParticipants(ctx context.Context, req *contract.ListParticipantsRequest) (*contract.ListParticipantsResponse, error)
	CreateAuditLog(ctx context.Context, req *contract.CreateAuditLogRequest) (*contract.CreateAuditLogResponse, error)
	GetAuditLogs(ctx context.Context, req *contract.GetAuditLogsRequest) (*contract.GetAuditLogsResponse, error)
	IssueParticipantCertificate(ctx context.Context, req *contract.IssueParticipantCertificateRequest) (*contract.IssueParticipantCertificateResponse, error)
	SignParticipantCSR(ctx context.Context, req *contract.SignParticipantCSRRequest) (*contract.SignParticipantCSRResponse, error)
	ApproveKYC(ctx context.Context, req *contract.ApproveKYCRequest) (*contract.ApproveKYCResponse, error)
	ManageParticipantStatus(ctx context.Context, req *contract.ManageParticipantStatusRequest) (*contract.ManageParticipantStatusResponse, error)
	GetCircuitBreakerStatus(ctx context.Context, req *struct{}) (*contract.GetCircuitBreakerStatusResponse, error)
	ToggleCircuitBreaker(ctx context.Context, req *contract.ToggleCircuitBreakerRequest) (*contract.ToggleCircuitBreakerResponse, error)
	GetSystemParameters(ctx context.Context, req *struct{}) (*contract.GetSystemParametersResponse, error)
	UpdateSystemParameters(ctx context.Context, req *contract.UpdateSystemParametersRequest) (*contract.UpdateSystemParametersResponse, error)
}

// New builds a configured gRPC server with all compliance handlers.
// bc may be nil; when nil, a NoopRegistryClient is used (dev/test mode).
func New(repo repository.Repository, ca *compliancepki.CA, bc registry.RegistryWriter) *grpc.Server {
	if bc == nil {
		bc = registry.NoopRegistryClient{}
	}
	svc := &complianceService{repo: repo, ca: ca, blockchain: bc}
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*complianceServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "UpsertParticipant", Handler: upsertParticipantHandler},
			{MethodName: "GetParticipantByUser", Handler: getParticipantByUserHandler},
			{MethodName: "ListParticipants", Handler: listParticipantsHandler},
			{MethodName: "CreateAuditLog", Handler: createAuditLogHandler},
			{MethodName: "GetAuditLogs", Handler: getAuditLogsHandler},
		{MethodName: "IssueParticipantCertificate", Handler: issueParticipantCertificateHandler},
		{MethodName: "SignParticipantCSR", Handler: signParticipantCSRHandler},
		{MethodName: "ApproveKYC", Handler: approveKYCHandler},
			{MethodName: "ManageParticipantStatus", Handler: manageParticipantStatusHandler},
			{MethodName: "GetCircuitBreakerStatus", Handler: getCircuitBreakerStatusHandler},
			{MethodName: "ToggleCircuitBreaker", Handler: toggleCircuitBreakerHandler},
			{MethodName: "GetSystemParameters", Handler: getSystemParametersHandler},
			{MethodName: "UpdateSystemParameters", Handler: updateSystemParametersHandler},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "compliance.v1",
	}, svc)
	return grpcServer
}

// --- Participant ---

func (s *complianceService) UpsertParticipant(ctx context.Context, req *contract.UpsertParticipantRequest) (*contract.UpsertParticipantResponse, error) {
	if strings.TrimSpace(req.Participant.UserID) == "" {
		return nil, status.Error(codes.InvalidArgument, "participant.user_id is required")
	}
	p := repository.Participant{
		UserID:          req.Participant.UserID,
		InstitutionName: req.Participant.InstitutionName,
		CNPJ:            req.Participant.CNPJ,
		BankCode:        req.Participant.BankCode,
		CountryCode:     req.Participant.CountryCode,
		Role:            req.Participant.Role,
		WalletAddress:   req.Participant.WalletAddress,
		Status:          req.Participant.Status,
		CertificateData: req.Participant.CertificateData,
		CertificateExpiry: req.Participant.CertificateExpiry,
	}
	if p.Status == "" {
		p.Status = string(domain.StatusPending)
	}
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.UpsertParticipantResponse{Success: true}, nil
}

func (s *complianceService) GetParticipantByUser(ctx context.Context, req *contract.GetParticipantByUserRequest) (*contract.GetParticipantByUserResponse, error) {
	if strings.TrimSpace(req.UserID) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	p, found, err := s.repo.GetParticipantByUser(ctx, req.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &contract.GetParticipantByUserResponse{Found: false}, nil
	}
	return &contract.GetParticipantByUserResponse{
		Found:       true,
		Participant: participantToContract(p),
	}, nil
}

func (s *complianceService) ListParticipants(ctx context.Context, req *contract.ListParticipantsRequest) (*contract.ListParticipantsResponse, error) {
	list, err := s.repo.ListParticipants(ctx, repository.ParticipantFilter{
		Status: req.Status,
		Search: req.Search,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	result := make([]contract.Participant, len(list))
	for i, p := range list {
		result[i] = participantToContract(p)
	}
	return &contract.ListParticipantsResponse{Participants: result}, nil
}

// --- Audit Log ---

func (s *complianceService) CreateAuditLog(ctx context.Context, req *contract.CreateAuditLogRequest) (*contract.CreateAuditLogResponse, error) {
	entry := repository.AuditEntry{
		ActorSubject:  req.Entry.ActorSubject,
		ActorAddress:  req.Entry.ActorAddress,
		ActionType:    req.Entry.ActionType,
		TargetSubject: req.Entry.TargetSubject,
		CorrelationID: req.Entry.CorrelationID,
		IPAddress:     req.Entry.IPAddress,
		Result:        req.Entry.Result,
		Category:      req.Entry.Category,
		Severity:      req.Entry.Severity,
		Details:       req.Entry.Details,
	}
	if err := s.repo.CreateAuditLog(ctx, entry); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.CreateAuditLogResponse{Success: true}, nil
}

func (s *complianceService) GetAuditLogs(ctx context.Context, req *contract.GetAuditLogsRequest) (*contract.GetAuditLogsResponse, error) {
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
	logs := make([]contract.AuditLogRecord, len(records))
	for i, r := range records {
		logs[i] = contract.AuditLogRecord{
			LogID:         r.LogID,
			Timestamp:     r.Timestamp.UTC().Format(time.RFC3339),
			ActorSubject:  r.ActorSubject,
			ActorAddress:  r.ActorAddress,
			ActionType:    r.ActionType,
			TargetSubject: r.TargetSubject,
			CorrelationID: r.CorrelationID,
			Category:      r.Category,
			Severity:      r.Severity,
			Result:        r.Result,
			Details:       r.Details,
		}
	}
	return &contract.GetAuditLogsResponse{Logs: logs}, nil
}

// --- PKI ---

func (s *complianceService) IssueParticipantCertificate(ctx context.Context, req *contract.IssueParticipantCertificateRequest) (*contract.IssueParticipantCertificateResponse, error) {
	if req.UserID == "" || req.Role == "" || req.InstitutionName == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, role, and institution_name are required")
	}
	if s.ca == nil {
		return nil, status.Error(codes.Unimplemented, "CA not configured")
	}

	issued, err := s.ca.IssueParticipantCert(req.UserID, req.InstitutionName, req.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "issue certificate: %v", err)
	}

	expiresAt := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)

	s.emitAudit(ctx, "ISSUE_CERTIFICATE", req.UserID, "", req.UserID,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo),
		fmt.Sprintf(`{"role":%q,"institution":%q}`, req.Role, req.InstitutionName))

	return &contract.IssueParticipantCertificateResponse{
		CertPEM:    issued.CertPEM,
		PrivKeyPEM: issued.PrivKeyPEM,
		ExpiresAt:  expiresAt,
	}, nil
}

// SignParticipantCSR signs a PKCS#10 CSR submitted by a participant, updates
// only the certificate fields in the participant record, and (best-effort)
// registers on the blockchain using the wallet address set during onboarding.
//
// The participant's wallet address is set by the KMS during OnboardParticipant
// and must be preserved here — it is NOT derived from CB_PRIVATE_KEY.
func (s *complianceService) SignParticipantCSR(ctx context.Context, req *contract.SignParticipantCSRRequest) (*contract.SignParticipantCSRResponse, error) {
	if req.CSRPEM == "" || req.UserID == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "csr_pem, user_id, and role are required")
	}
	if s.ca == nil {
		return nil, status.Error(codes.Unimplemented, "CA not configured")
	}

	issued, err := s.ca.SignCSR(req.CSRPEM)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sign CSR: %v", err)
	}

	// Fetch existing record to preserve the KMS-generated wallet address and
	// other fields set during onboarding that are not present in the CSR request.
	existing, found, fetchErr := s.repo.GetParticipantByUser(ctx, req.UserID)
	if fetchErr != nil {
		return nil, status.Errorf(codes.Internal, "sign CSR: lookup participant: %v", fetchErr)
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "participant %q not found; call POST /compliance/register first", req.UserID)
	}
	if existing.WalletAddress == "" {
		log.Printf("WARN: SignParticipantCSR: participant %s has no wallet address — ApproveKYC will fail until wallet is set", req.UserID)
	}

	expiresAt := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)

	institutionName := req.InstitutionName
	if institutionName == "" {
		institutionName = existing.InstitutionName
	}
	cnpj := req.CNPJ
	if cnpj == "" {
		cnpj = existing.CNPJ
	}

	p := repository.Participant{
		UserID:          req.UserID,
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
		if _, err := s.blockchain.SetParticipant(ctx, existing.WalletAddress, req.Role, true); err != nil {
			log.Printf("WARN: SignParticipantCSR: on-chain registration failed (non-fatal): %v", err)
		}
	}

	s.emitAudit(ctx, "SIGN_CSR", actorFromCtx(ctx), "", req.UserID,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo),
		fmt.Sprintf(`{"role":%q,"institution":%q}`, req.Role, req.InstitutionName))

	return &contract.SignParticipantCSRResponse{
		CertPEM:   issued.CertPEM,
		ExpiresAt: expiresAt,
	}, nil
}

// --- Governance ---

// ApproveKYC sets the participant status to ACTIVE in PostgreSQL and calls
// setParticipant(wallet, role, true) on the ParticipantRegistry contract.
// This is the canonical "approve" operation for the PKI-based login flow.
func (s *complianceService) ApproveKYC(ctx context.Context, req *contract.ApproveKYCRequest) (*contract.ApproveKYCResponse, error) {
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
	if p.WalletAddress == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "participant %q has no wallet address", req.Subject)
	}

	// Activate on-chain first; the DB update is the commit point.
	txHash, err := s.blockchain.SetParticipant(ctx, p.WalletAddress, p.Role, true)
	if err != nil {
		log.Printf("WARN: ApproveKYC: on-chain activation failed for %s: %v", req.Subject, err)
		txHash = ""
	}

	p.Status = string(domain.StatusActive)
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	detailsJSON, _ := json.Marshal(map[string]string{
		"status":  string(domain.StatusActive),
		"reason":  req.Reason,
		"tx_hash": txHash,
	})
	s.emitAudit(ctx, "APPROVE_KYC", req.ActorSubject, "", req.Subject,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS",
		string(domain.CategoryCredential), string(domain.SeverityInfo), string(detailsJSON))

	return &contract.ApproveKYCResponse{
		Subject: req.Subject,
		Status:  string(domain.StatusActive),
		TxHash:  txHash,
	}, nil
}

func (s *complianceService) ManageParticipantStatus(ctx context.Context, req *contract.ManageParticipantStatusRequest) (*contract.ManageParticipantStatusResponse, error) {
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

	return &contract.ManageParticipantStatusResponse{Subject: req.Subject, Status: req.Status}, nil
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

func (s *complianceService) GetCircuitBreakerStatus(ctx context.Context, _ *struct{}) (*contract.GetCircuitBreakerStatusResponse, error) {
	paused, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerPaused)
	updatedBy, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedBy)
	updatedAt, _, _ := s.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedAt)

	return &contract.GetCircuitBreakerStatusResponse{
		IsPaused:   paused == "true",
		LastUpdate: updatedAt,
		UpdatedBy:  updatedBy,
	}, nil
}

func (s *complianceService) ToggleCircuitBreaker(ctx context.Context, req *contract.ToggleCircuitBreakerRequest) (*contract.ToggleCircuitBreakerResponse, error) {
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

	return &contract.ToggleCircuitBreakerResponse{IsPaused: req.Pause}, nil
}

func (s *complianceService) GetSystemParameters(ctx context.Context, _ *struct{}) (*contract.GetSystemParametersResponse, error) {
	txMin, _, _ := s.repo.GetSystemParameter(ctx, paramTxMinimum)
	txMax, _, _ := s.repo.GetSystemParameter(ctx, paramTxMaximum)
	slippageStr, _, _ := s.repo.GetSystemParameter(ctx, paramSlippage)
	settlementStr, _, _ := s.repo.GetSystemParameter(ctx, paramSettlementWindow)

	slippage, _ := strconv.ParseFloat(slippageStr, 64)
	settlement, _ := strconv.ParseInt(settlementStr, 10, 64)

	return &contract.GetSystemParametersResponse{
		TransactionMinimum: txMin,
		TransactionMaximum: txMax,
		SlippageTolerance:  slippage,
		SettlementWindow:   settlement,
	}, nil
}

func (s *complianceService) UpdateSystemParameters(ctx context.Context, req *contract.UpdateSystemParametersRequest) (*contract.UpdateSystemParametersResponse, error) {
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

	return &contract.UpdateSystemParametersResponse{Success: true}, nil
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

func participantToContract(p repository.Participant) contract.Participant {
	return contract.Participant{
		UserID:            p.UserID,
		InstitutionName:   p.InstitutionName,
		CNPJ:              p.CNPJ,
		BankCode:          p.BankCode,
		CountryCode:       p.CountryCode,
		Role:              p.Role,
		WalletAddress:     p.WalletAddress,
		Status:            p.Status,
		CertificateData:   p.CertificateData,
		CertificateExpiry: p.CertificateExpiry,
	}
}

// --- gRPC handler adapters ---

func upsertParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.UpsertParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).UpsertParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.UpsertParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).UpsertParticipant(ctx, req.(*contract.UpsertParticipantRequest))
	})
}

func getParticipantByUserHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetParticipantByUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).GetParticipantByUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetParticipantByUserMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).GetParticipantByUser(ctx, req.(*contract.GetParticipantByUserRequest))
	})
}

func listParticipantsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ListParticipantsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).ListParticipants(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ListParticipantsMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).ListParticipants(ctx, req.(*contract.ListParticipantsRequest))
	})
}

func createAuditLogHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.CreateAuditLogRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).CreateAuditLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.CreateAuditLogMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).CreateAuditLog(ctx, req.(*contract.CreateAuditLogRequest))
	})
}

func getAuditLogsHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetAuditLogsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).GetAuditLogs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetAuditLogsMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).GetAuditLogs(ctx, req.(*contract.GetAuditLogsRequest))
	})
}

func issueParticipantCertificateHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.IssueParticipantCertificateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).IssueParticipantCertificate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.IssueParticipantCertificateMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).IssueParticipantCertificate(ctx, req.(*contract.IssueParticipantCertificateRequest))
	})
}

func signParticipantCSRHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.SignParticipantCSRRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).SignParticipantCSR(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.SignParticipantCSRMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).SignParticipantCSR(ctx, req.(*contract.SignParticipantCSRRequest))
	})
}

func approveKYCHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ApproveKYCRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).ApproveKYC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ApproveKYCMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).ApproveKYC(ctx, req.(*contract.ApproveKYCRequest))
	})
}

func manageParticipantStatusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ManageParticipantStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).ManageParticipantStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ManageParticipantStatusMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).ManageParticipantStatus(ctx, req.(*contract.ManageParticipantStatusRequest))
	})
}

func getCircuitBreakerStatusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(struct{})
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).GetCircuitBreakerStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetCircuitBreakerStatusMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).GetCircuitBreakerStatus(ctx, req.(*struct{}))
	})
}

func toggleCircuitBreakerHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ToggleCircuitBreakerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).ToggleCircuitBreaker(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ToggleCircuitBreakerMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).ToggleCircuitBreaker(ctx, req.(*contract.ToggleCircuitBreakerRequest))
	})
}

func getSystemParametersHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(struct{})
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).GetSystemParameters(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetSystemParametersMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).GetSystemParameters(ctx, req.(*struct{}))
	})
}

func updateSystemParametersHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.UpdateSystemParametersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*complianceService).UpdateSystemParameters(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.UpdateSystemParametersMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*complianceService).UpdateSystemParameters(ctx, req.(*contract.UpdateSystemParametersRequest))
	})
}
