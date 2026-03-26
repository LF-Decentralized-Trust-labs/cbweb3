package server

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SubmitCredentialRequest handles Phase 1 of the new onboarding flow.
// The commercial bank submits a CSR (P-256) and blockchain public key (secp256k1).
// The Central Bank validates both, creates a Keycloak user, and persists the
// participant with status CREDENTIAL_REQUESTED.
func (s *identityService) SubmitCredentialRequest(ctx context.Context, req *authv1.SubmitCredentialRequestReq) (*authv1.SubmitCredentialRequestResp, error) {
	if req.CsrPem == "" || req.BlockchainPubKeyHex == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "csr_pem, blockchain_pub_key_hex, and role are required")
	}
	if req.Username == "" || req.Email == "" || req.InstitutionName == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email, and institution_name are required")
	}

	// 1. Validate CSR signature (P-256).
	block, _ := pem.Decode([]byte(req.CsrPem))
	if block == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse CSR: %v", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "CSR signature verification failed")
	}

	// 2. Derive wallet address from blockchain public key.
	walletAddr, err := pki.DeriveAddress(req.BlockchainPubKeyHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "derive wallet address: %v", err)
	}

	// 3. Create Keycloak user.
	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "credential request: admin token: %v", err)
	}

	displayName := req.InstitutionName
	userID, err := s.keycloak.CreateUser(ctx, adminToken, keycloak.CreateUserRequest{
		Username:      req.Username,
		Email:         req.Email,
		FirstName:     displayName,
		LastName:      displayName,
		EmailVerified: true,
		Enabled:       true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "credential request: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "credential request: creating keycloak user: %v", err)
	}

	if roleErr := s.keycloak.AssignRealmRole(ctx, adminToken, userID, req.Role); roleErr != nil {
		_ = roleErr // non-fatal
	}

	// 4. Persist participant with status CREDENTIAL_REQUESTED.
	if upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:              userID,
		WalletAddress:       walletAddr,
		CountryCode:         req.Country,
		BankCode:            req.BankCode,
		Role:                req.Role,
		InstitutionName:     req.InstitutionName,
		CNPJ:                req.Cnpj,
		Status:              string(domain.ParticipantStatusCredentialRequested),
		BlockchainPubKeyHex: req.BlockchainPubKeyHex,
		CsrPem:              req.CsrPem,
	}); upsertErr != nil {
		if isConflict(upsertErr) {
			return nil, status.Error(codes.AlreadyExists, upsertErr.Error())
		}
		return nil, status.Errorf(codes.Internal, "credential request: persisting participant: %v", upsertErr)
	}

	s.emitAudit(ctx, "SUBMIT_CREDENTIAL_REQUEST", userID, walletAddr, "",
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")

	return &authv1.SubmitCredentialRequestResp{
		RequestId:     userID,
		UserId:        userID,
		WalletAddress: walletAddr,
		Status:        string(domain.ParticipantStatusCredentialRequested),
	}, nil
}

// GetOnboardingStatus handles Phase 2.5 — the commercial bank polls this endpoint
// to discover when KYC has been approved and to retrieve the PoP nonce.
func (s *identityService) GetOnboardingStatus(ctx context.Context, req *authv1.GetOnboardingStatusRequest) (*authv1.GetOnboardingStatusResponse, error) {
	if req.RequestId == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}

	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "onboarding status: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "onboarding request not found")
	}

	resp := &authv1.GetOnboardingStatusResponse{
		RequestId:     req.RequestId,
		UserId:        participant.UserID,
		Status:        participant.Status,
		WalletAddress: participant.WalletAddress,
	}

	// Only expose pop_nonce when KYC is approved and nonce is not expired.
	if participant.Status == string(domain.ParticipantStatusKYCApproved) && participant.PopNonce != "" {
		if participant.PopNonceExpiresAt == nil || time.Now().UTC().Before(*participant.PopNonceExpiresAt) {
			resp.PopNonce = participant.PopNonce
		}
	}

	return resp, nil
}

// CompleteOnboarding handles Phase 3 — the commercial bank signs the PoP nonce
// with its secp256k1 key, proving wallet ownership. On success, the CA issues
// an X.509 certificate with the wallet extension, the wallet is registered
// on-chain, and the participant becomes ACTIVE.
func (s *identityService) CompleteOnboarding(ctx context.Context, req *authv1.CompleteOnboardingRequest) (*authv1.CompleteOnboardingResponse, error) {
	if req.UserId == "" || req.PopSignatureHex == "" || req.BlockchainPubKeyHex == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, pop_signature_hex, and blockchain_pub_key_hex are required")
	}

	// 1. Fetch participant and validate state.
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "participant not found")
	}
	if participant.Status != string(domain.ParticipantStatusKYCApproved) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"participant status is %q; expected KYC_APPROVED", participant.Status)
	}

	// 2. Validate PoP nonce is not expired.
	if participant.PopNonce == "" {
		return nil, status.Error(codes.FailedPrecondition, "no PoP nonce found; re-approve KYC")
	}
	if participant.PopNonceExpiresAt != nil && time.Now().UTC().After(*participant.PopNonceExpiresAt) {
		return nil, status.Error(codes.DeadlineExceeded, "PoP nonce expired; re-approve KYC")
	}

	// 3. Verify Proof of Possession (secp256k1).
	if err := pki.VerifyPoP(participant.PopNonce, req.PopSignatureHex, participant.WalletAddress); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "PoP verification failed: %v", err)
	}

	// 4. Validate that submitted pub key matches stored wallet address.
	if err := pki.ValidatePubKeyMatchesAddress(req.BlockchainPubKeyHex, participant.WalletAddress); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "public key mismatch: %v", err)
	}

	// 5. Sign the stored CSR to issue certificate preserving the participant's public key.
	if participant.CsrPem == "" {
		return nil, status.Error(codes.FailedPrecondition, "no CSR found for participant; re-submit credential request")
	}
	certResult, certErr := s.compliance.SignParticipantCSR(ctx,
		participant.CsrPem, participant.UserID, participant.Role, participant.InstitutionName, participant.CNPJ)
	if certErr != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: sign CSR: %v", certErr)
	}

	// 6. Register on-chain.
	// Only the Central Bank executes CompleteOnboarding — commercial banks
	// proxy via CENTRAL_BANK_API_URL. Signing uses the static CB_PRIVATE_KEY.
	var txHash string
	if domain.RequiresOnChain(participant.Role) && participant.WalletAddress != "" {
		hash, chainErr := s.blockchainClient.RegisterParticipant(ctx, participant.WalletAddress, participant.InstitutionName, participant.Role, [32]byte{})
		if chainErr != nil {
			return nil, status.Errorf(codes.Internal, "complete onboarding: on-chain registration: %v", chainErr)
		}
		txHash = hash
	}

	// 7. Generate client secret.
	rawBytes := make([]byte, 32)
	if _, rndErr := rand.Read(rawBytes); rndErr != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: generate secret: %v", rndErr)
	}
	rawSecret := base64.StdEncoding.EncodeToString(rawBytes)

	adminToken, adminErr := s.keycloak.GetAdminToken(ctx)
	if adminErr != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: admin token: %v", adminErr)
	}
	if pErr := s.keycloak.ResetPassword(ctx, adminToken, req.UserId, rawSecret); pErr != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: set password: %v", pErr)
	}

	// 8. Update participant to ACTIVE, clear PoP nonce, store certificate.
	if upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:          participant.UserID,
		WalletAddress:   participant.WalletAddress,
		CountryCode:     participant.CountryCode,
		BankCode:        participant.BankCode,
		Role:            participant.Role,
		InstitutionName: participant.InstitutionName,
		CNPJ:            participant.CNPJ,
		Status:          string(domain.ParticipantStatusActive),
		CertificateData: certResult.CertPEM,
		PopNonce:        "",
	}); upsertErr != nil {
		return nil, status.Errorf(codes.Internal, "complete onboarding: update participant: %v", upsertErr)
	}

	s.emitAudit(ctx, "COMPLETE_ONBOARDING", req.UserId, participant.WalletAddress, "",
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")

	return &authv1.CompleteOnboardingResponse{
		UserId:        req.UserId,
		WalletAddress: participant.WalletAddress,
		CertPem:       certResult.CertPEM,
		TxHash:        txHash,
		ClientSecret:  rawSecret,
		Status:        string(domain.ParticipantStatusActive),
	}, nil
}
