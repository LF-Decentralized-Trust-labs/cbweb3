// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	compliancepki "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
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
//
// R2-H-8: the server installs authorization interceptors (and mutual TLS when the
// GRPC_MTLS_* env vars are set). With nothing set it runs in audit mode with NO
// caller authentication so existing plaintext callers keep working; the
// x-caller-identity header is trusted only under GRPC_AUTHZ_ALLOW_HEADER_IDENTITY
// (transitional). GRPC_AUTHZ_ENFORCE (which requires mTLS) rejects unauthenticated
// callers. An error is returned on a fail-open misconfiguration (partial mTLS
// material, or enforcement requested without mTLS).
func New(repo repository.Repository, ca *compliancepki.CA, bc registry.RegistryWriter) (*grpc.Server, error) {
	if bc == nil {
		bc = registry.NoopRegistryClient{}
	}
	svc := &complianceService{repo: repo, ca: ca, blockchain: bc}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	serverOpts, err := authz.ServerOptionsFromEnv(logger, serverPolicy())
	if err != nil {
		return nil, fmt.Errorf("configure gRPC security: %w", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	compliancv1.RegisterComplianceServiceServer(grpcServer, svc)
	return grpcServer, nil
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
		LegalEntityID:       req.Participant.LegalEntityId,
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
	filter := repository.ParticipantFilter{Status: req.Status}
	// Callers may pass "bank_code:<code>" in the Search field to request an
	// exact bank_code match without requiring a new proto field.
	if strings.HasPrefix(req.Search, "bank_code:") {
		filter.BankCode = strings.TrimPrefix(req.Search, "bank_code:")
	} else {
		filter.Search = req.Search
	}
	list, err := s.repo.ListParticipants(ctx, filter)
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

// RegisterParticipantOnChain registers a wallet on the IdentityRegistry directly
// (the configured signer must hold GOVERNANCE_ROLE). It is the machine-to-machine
// self-registration path — no CSR nor prior DB record required — used e.g. by a
// founding central bank registering its spoke on the neutral hub. Idempotent:
// skips when the wallet can already transact. Mirrors the participant into the
// repo for audit (best-effort).
func (s *complianceService) RegisterParticipantOnChain(ctx context.Context, req *compliancv1.RegisterParticipantOnChainRequest) (*compliancv1.RegisterParticipantOnChainResponse, error) {
	wallet := strings.TrimSpace(req.WalletAddress)
	if wallet == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_address is required")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "ROLE_CENTRAL_BANK"
	}
	// Idempotent: an already-transacting wallet is treated as registered. The
	// live Besu client also implements RegistryReader; the noop client does not.
	if reader, ok := s.blockchain.(registry.RegistryReader); ok {
		if can, err := reader.CanTransact(ctx, wallet); err == nil && can {
			return &compliancv1.RegisterParticipantOnChainResponse{AlreadyRegistered: true}, nil
		}
	}
	// Two-step onboarding (R1-10.6 / R2-10.6): register (Pending) then verify (Verified) so the
	// wallet can transact. EnsureVerifiedParticipant is idempotent and never demotes an already
	// transactable wallet.
	txHash, err := registry.EnsureVerifiedParticipant(ctx, s.blockchain, wallet, req.InstitutionName, role, [32]byte{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "on-chain register+verify participant: %v", err)
	}
	// Best-effort DB mirror (never fails the on-chain result).
	_ = s.repo.UpsertParticipant(ctx, repository.Participant{
		UserID:          wallet,
		InstitutionName: req.InstitutionName,
		Role:            role,
		Status:          "ACTIVE",
		WalletAddress:   wallet,
		BankCode:        req.BankCode,
	})
	return &compliancv1.RegisterParticipantOnChainResponse{TxHash: txHash}, nil
}

// currencyRegistrar is the subset of the live Besu client used for sovereign
// currency registration. It is satisfied by the concrete *registry.BesuClient
// but NOT by the noop client, so a type assertion lets the RPC degrade
// gracefully (codes.Unimplemented) in dev/test mode.
type currencyRegistrar interface {
	RegisterCurrency(ctx context.Context, tokenName, tokenSymbol, countryName, proposerCB, cbAddress string) (tokenAddr string, txHash string, err error)
	IsCurrencyRegistered(ctx context.Context, symbol string) (bool, error)
	CurrencyTokenAddress(ctx context.Context, symbol string) (string, error)
	// EnsureCurrencyAuthority completes the handover of an already-registered currency's
	// issuance authority to its central bank. Idempotent; returns "" when nothing was due.
	EnsureCurrencyAuthority(ctx context.Context, tokenAddress, cbAddress string) (string, error)
}

// RegisterCurrencyOnChain deploys a founding central bank's bridge token
// (W-token) and registers its sovereign currency on-chain. Mirrors
// RegisterParticipantOnChain: the hub compliance signer (hub admin == the CB in
// local) performs the on-chain work. Idempotent by W-token symbol.
func (s *complianceService) RegisterCurrencyOnChain(ctx context.Context, req *compliancv1.RegisterCurrencyOnChainRequest) (*compliancv1.RegisterCurrencyOnChainResponse, error) {
	cbAddress := strings.TrimSpace(req.CbAddress)
	if cbAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "cb_address is required")
	}
	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}

	symbol := "W-tCeBM_" + currency
	name := "Wrapped tCeBM " + currency
	country := "Sovereign " + currency
	proposerCB := strings.TrimSpace(req.SpokeId)
	if proposerCB == "" {
		proposerCB = currency
	}

	// The blockchain field is a write-only RegistryWriter; the currency methods
	// live on the concrete Besu client only. The noop client does not implement
	// currencyRegistrar → return Unimplemented in dev/test mode.
	reg, ok := s.blockchain.(currencyRegistrar)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "currency registration is not available (no on-chain signer configured)")
	}

	// Idempotent: skip when the currency is already registered on-chain — but still
	// resolve + return the token address so callers can wire W_TOKEN_ADDRESS on re-runs.
	if already, err := reg.IsCurrencyRegistered(ctx, symbol); err == nil && already {
		addr, _ := reg.CurrencyTokenAddress(ctx, symbol)
		// Registration and the handover of issuance authority to the CB are separate
		// transactions, so a run interrupted between them leaves a currency that only the hub
		// can mint. Converge here instead of reporting success on a half-done handover.
		if addr != "" {
			if txHash, hErr := reg.EnsureCurrencyAuthority(ctx, addr, cbAddress); hErr != nil {
				return nil, status.Errorf(codes.Internal, "currency %s is registered but its issuance authority is not with %s: %v", symbol, cbAddress, hErr)
			} else if txHash != "" {
				log.Printf("[compliance] currency %s: completed issuance-authority handover to %s (tx=%s)", symbol, cbAddress, txHash)
			}
		}
		return &compliancv1.RegisterCurrencyOnChainResponse{Symbol: symbol, TokenAddress: addr, AlreadyRegistered: true}, nil
	}

	tokenAddr, txHash, err := reg.RegisterCurrency(ctx, name, symbol, country, proposerCB, cbAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "on-chain registerCurrency: %v", err)
	}
	return &compliancv1.RegisterCurrencyOnChainResponse{
		Symbol:       symbol,
		TokenAddress: tokenAddr,
		TxHash:       txHash,
	}, nil
}

// pairRegistrar is the subset of the live Besu client used for sovereign-pair
// registration (deploy AMM + proposePair + confirmPair). Satisfied by the
// concrete *registry.BesuClient but not the noop client.
type pairRegistrar interface {
	RegisterPair(ctx context.Context, symbolA, symbolB, pairID string) (ammAddr string, txHash string, err error)
	IsPairRegistered(ctx context.Context, pairID string) (bool, error)
}

// RegisterPairOnChain deploys the sovereign-pair AMM over the two already-
// registered W-tokens and registers the pair (proposePair + confirmPair) via the
// hub compliance signer. Idempotent by pair id. Mirrors RegisterCurrencyOnChain.
func (s *complianceService) RegisterPairOnChain(ctx context.Context, req *compliancv1.RegisterPairOnChainRequest) (*compliancv1.RegisterPairOnChainResponse, error) {
	ca := strings.TrimSpace(req.CurrencyA)
	cb := strings.TrimSpace(req.CurrencyB)
	if ca == "" || cb == "" {
		return nil, status.Error(codes.InvalidArgument, "currency_a and currency_b are required")
	}
	pairID := strings.TrimSpace(req.PairId)
	if pairID == "" {
		// Convention W-{source}-W-{target} — must match the swap/quote path
		// (swap_quote_generator builds "W-%s-W-%s"), else the on-chain per-pair
		// resolver misses the pool.
		pairID = "W-" + ca + "-W-" + cb
	}
	symbolA := "W-tCeBM_" + ca
	symbolB := "W-tCeBM_" + cb

	reg, ok := s.blockchain.(pairRegistrar)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "pair registration is not available (no on-chain signer configured)")
	}

	// Idempotent: skip when the pair already exists on-chain.
	if already, err := reg.IsPairRegistered(ctx, pairID); err == nil && already {
		return &compliancv1.RegisterPairOnChainResponse{AlreadyRegistered: true}, nil
	}

	ammAddr, txHash, err := reg.RegisterPair(ctx, symbolA, symbolB, pairID)
	if err != nil {
		// Once each currency's issuance authority rests with its own central bank, the
		// PairRegistry admits only those two as proposer and confirmer — the hub is neither.
		// FailedPrecondition (not Internal) with the AMM address and the addresses that owe
		// each act, so the caller routes the request to the CBs' own
		// POST /api/v2/amm/pairs/{propose,confirm} instead of retrying here.
		var awaits *registry.PairAwaitsSovereignsError
		if errors.As(err, &awaits) {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", awaits)
		}
		return nil, status.Errorf(codes.Internal, "on-chain registerPair: %v", err)
	}
	return &compliancv1.RegisterPairOnChainResponse{
		AmmAddress: ammAddr,
		TxHash:     txHash,
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
	legalEntityID := req.LegalEntityId
	if legalEntityID == "" {
		legalEntityID = existing.LegalEntityID
	}

	p := repository.Participant{
		UserID:          req.UserId,
		InstitutionName: institutionName,
		LegalEntityID:   legalEntityID,
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
		// Two-step onboarding (R1-10.6 / R2-10.6), best-effort: register (Pending) then verify
		// (Verified). EnsureVerifiedParticipant is idempotent and never demotes — this path is
		// re-run on repeated CSR signings ("may already be approved"), so a live, transactable
		// wallet is left untouched rather than reset to Pending mid-flight.
		if _, err := registry.EnsureVerifiedParticipant(ctx, s.blockchain, existing.WalletAddress, institutionName, req.Role, [32]byte{}); err != nil {
			log.Printf("WARN: SignParticipantCSR: on-chain register+verify failed (non-fatal): %v", err)
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
	s.emitAudit(ctx, "KYC_APPROVED", actorForAudit(ctx, req.ActorSubject), "", req.Subject,
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
	if req.Status == "KYC_REJECTED" {
		p.RejectionReason = req.Reason
	}
	if err := s.repo.UpsertParticipant(ctx, p); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sev := string(domain.SeverityWarning)
	if req.Status == string(domain.StatusRevoked) {
		sev = string(domain.SeverityCritical)
	}
	detailsJSON, _ := json.Marshal(map[string]string{"status": req.Status, "reason": req.Reason})
	action := "MANAGE_PARTICIPANT_STATUS"
	if req.Status == "KYC_REJECTED" {
		action = "KYC_REJECTED"
	}
	s.emitAudit(ctx, action, "", "", req.Subject,
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
	// R2-H-8: prefer the authenticated caller identity over the payload actor,
	// which is spoofable.
	actor := actorForAudit(ctx, req.ActorSubject)

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

// actorFromCtx returns the caller identity for audit attribution: the identity the
// gRPC authz interceptor authenticated (mTLS peer certificate, or the trusted
// metadata header when that transitional mode is explicitly opted into).
//
// The legacy x-actor-subject header was removed here (R2-H-8 follow-up item 3). It
// was a fallback no gateway set, and any peer that could reach the port could set
// it — so the one thing it could still do was let an unauthenticated caller choose
// the name written to the compliance audit trail. Removing it costs nothing real and
// closes an attacker-writable channel; when no identity is authenticated the caller
// now gets no actor from the transport at all, and the audit falls back to the
// gateway-validated payload (see actorForAudit).
func actorFromCtx(ctx context.Context) string {
	return authz.Actor(ctx)
}

// actorForAudit derives the audit actor, preferring the authenticated caller
// identity over any actor value supplied in the request payload. R2-H-8: the
// payload actor is caller-controlled and therefore spoofable; it is used only as
// a last-resort fallback during the pre-mTLS transition. Under enforcement + mTLS
// the authenticated identity is always present, so the payload value is ignored.
func actorForAudit(ctx context.Context, payloadActor string) string {
	if a := actorFromCtx(ctx); a != "" {
		return a
	}
	return payloadActor
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
		LegalEntityId:       p.LegalEntityID,
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
