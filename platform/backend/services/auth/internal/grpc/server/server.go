package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/pki"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/blockchain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// nonce store — TTL-based in-memory map for PKI login step 1.
// For production, replace with Redis.
type nonceEntry struct {
	nonce     string
	expiresAt time.Time
}

type identityService struct {
	keycloak         keycloak.Client
	kms              kms.Provider
	compliance       complianceclient.Client
	blockchainClient blockchain.Client
	caCertPEM        string // Central Bank CA cert PEM for PKI verification

	nonceMu sync.Mutex
	nonces  map[string]nonceEntry // userID → nonceEntry
}

type identityServiceServer interface {
	Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error)
	RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error)
	RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error)
	ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error)
	RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error)
	GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error)
	ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error)
	OnboardParticipant(ctx context.Context, req *contract.OnboardParticipantRequest) (*contract.OnboardParticipantResponse, error)
	IssueLoginNonce(ctx context.Context, req *contract.IssueLoginNonceRequest) (*contract.IssueLoginNonceResponse, error)
	VerifyPKILogin(ctx context.Context, req *contract.VerifyPKILoginRequest) (*contract.VerifyPKILoginResponse, error)
}

// New builds a configured gRPC server and registers all identity handlers.
func New(kc keycloak.Client, kmsProvider kms.Provider, compliance complianceclient.Client, bc blockchain.Client, caCertPEM string) *grpc.Server {
	svc := &identityService{
		keycloak:         kc,
		kms:              kmsProvider,
		compliance:       compliance,
		blockchainClient: bc,
		caCertPEM:        caCertPEM,
		nonces:           make(map[string]nonceEntry),
	}
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*identityServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: loginHandler},
			{MethodName: "RefreshToken", Handler: refreshTokenHandler},
			{MethodName: "RevokeToken", Handler: revokeTokenHandler},
			{MethodName: "ValidateToken", Handler: validateTokenHandler},
			{MethodName: "RegisterParticipant", Handler: registerParticipantHandler},
			{MethodName: "SignTransaction", Handler: signTransactionHandler},
			{MethodName: "GetKYCStatus", Handler: getKYCStatusHandler},
			{MethodName: "ProvisionParticipant", Handler: provisionParticipantHandler},
			{MethodName: "OnboardParticipant", Handler: onboardParticipantHandler},
			{MethodName: "IssueLoginNonce", Handler: issueLoginNonceHandler},
			{MethodName: "VerifyPKILogin", Handler: verifyPKILoginHandler},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "identity.v1",
	}, svc)
	return grpcServer
}

// --- Auth handlers ---

func (s *identityService) Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error) {
	tr, err := s.keycloak.Login(ctx, req.User, req.Password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	s.emitAudit(ctx, "LOGIN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.LoginResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
	}, nil
}

func (s *identityService) RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error) {
	tr, err := s.keycloak.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &contract.RefreshTokenResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
	}, nil
}

func (s *identityService) RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error) {
	// NOTE: Keycloak's logout endpoint requires the refresh_token, but this
	// gRPC method only receives an access_token field. Callers should send
	// the refresh_token in the AccessToken field until this is reconciled.
	if err := s.keycloak.Logout(ctx, req.AccessToken); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "REVOKE_TOKEN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.RevokeTokenResponse{}, nil
}

func (s *identityService) ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error) {
	claims, err := s.keycloak.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &contract.ValidateTokenResponse{
		Subject:      claims.Subject,
		Issuer:       claims.Issuer,
		Roles:        claims.Roles,
		Wallet:       claims.Wallet,
		Country:      claims.Country,
		BankID:       claims.BankID,
		PrivacyGroup: claims.PrivacyGroup,
	}, nil
}

// --- Participant Registration ---

func (s *identityService) RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error) {
	claims, err := s.keycloak.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	keyInfo, err := s.kms.CreateKey(ctx, claims.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:          claims.Subject,
		WalletAddress:   keyInfo.Address,
		CountryCode:     req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Status:          string(domain.ParticipantStatusPending),
	})
	if upsertErr != nil {
		if isConflict(upsertErr) {
			return nil, status.Error(codes.AlreadyExists, upsertErr.Error())
		}
		return nil, status.Error(codes.Internal, upsertErr.Error())
	}

	s.emitAudit(ctx, "REGISTER_PARTICIPANT", claims.Subject, keyInfo.Address, "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.RegisterParticipantResponse{
		UserID:        claims.Subject,
		WalletAddress: keyInfo.Address,
	}, nil
}

// --- Signing ---

func (s *identityService) SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error) {
	result, err := s.kms.Sign(ctx, req.UserID, req.Digest)
	if err != nil {
		if errors.Is(err, kms.ErrKeyNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.SignTransactionResponse{
		Signature: result.Signature,
		Address:   result.Address,
	}, nil
}

// --- KYC Status ---

func (s *identityService) GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error) {
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &contract.GetKYCStatusResponse{
			Subject: req.Subject,
			Status:  string(domain.ParticipantStatusPending),
		}, nil
	}
	st := participant.Status
	if st == "" {
		st = string(domain.ParticipantStatusPending)
	}
	return &contract.GetKYCStatusResponse{
		Subject: req.Subject,
		Status:  st,
	}, nil
}

func (s *identityService) ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error) {
	if err := s.compliance.ManageParticipantStatus(ctx, req.Subject, req.Status, "provisioned via identity service"); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "PROVISION_PARTICIPANT", "", "", req.Subject, correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.ProvisionParticipantResponse{}, nil
}

// --- Administrative Participant Onboarding ---

// OnboardParticipant is called by the Central Bank to register a new
// Commercial Bank or Treasury user. The flow is:
//
//  1. Obtain a Keycloak admin token via service-account client_credentials.
//  2. Create the user in Keycloak.
//  3. Conditionally generate a KMS key pair (roles requiring KMS).
//  4. Conditionally register the wallet address on-chain via ParticipantRegistry
//     (roles requiring on-chain registration, signed with CB_PRIVATE_KEY).
//  5. Persist the participant record via data-access service.
func (s *identityService) OnboardParticipant(ctx context.Context, req *contract.OnboardParticipantRequest) (*contract.OnboardParticipantResponse, error) {
	if req.Username == "" || req.Email == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email, and role are required")
	}
	if !domain.IsAdminRole(req.Role) {
		return nil, status.Errorf(codes.InvalidArgument, "role %q is not a valid admin-onboarded role", req.Role)
	}

	// 1. Obtain Keycloak admin token.
	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "onboard: obtaining admin token: %v", err)
	}

	// 2. Create user in Keycloak.
	userID, err := s.keycloak.CreateUser(ctx, adminToken, keycloak.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Enabled:  true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "onboard: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "onboard: creating keycloak user: %v", err)
	}

	resp := &contract.OnboardParticipantResponse{UserID: userID}

	// 3. Optionally generate KMS key.
	if domain.RequiresKMS(req.Role) {
		keyInfo, kmsErr := s.kms.CreateKey(ctx, userID)
		if kmsErr != nil {
			return nil, status.Errorf(codes.Internal, "onboard: creating kms key: %v", kmsErr)
		}
		resp.WalletAddress = keyInfo.Address
		resp.DID = keyInfo.DID
	}

	// 4. Optionally register on-chain.
	if domain.RequiresOnChain(req.Role) && resp.WalletAddress != "" {
		txHash, chainErr := s.blockchainClient.RegisterMember(ctx, resp.WalletAddress, req.Role)
		if chainErr != nil {
			return nil, status.Errorf(codes.Internal, "onboard: on-chain registration: %v", chainErr)
		}
		resp.TxHash = txHash
	}

	// 4b. Issue PKI certificate for roles that require it.
	if domain.RequiresPKI(req.Role) {
		issued, certErr := s.compliance.IssueParticipantCertificate(ctx, userID, req.Role, req.InstitutionName, req.BankCode)
		if certErr != nil {
			log.Printf("WARN: onboard: issue certificate for %s: %v (continuing)", userID, certErr)
		} else {
			resp.DID = issued.CertPEM // store cert PEM as the "identity credential"
		}
	}

	// 5. Persist participant record.
	if upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:          userID,
		WalletAddress:   resp.WalletAddress,
		CertificateData: resp.DID,
		CountryCode:     req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Status:          string(domain.ParticipantStatusPending),
	}); upsertErr != nil {
		return nil, status.Errorf(codes.Internal, "onboard: persisting participant: %v", upsertErr)
	}

	s.emitAudit(ctx, "ONBOARD_PARTICIPANT", userID, resp.WalletAddress, "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return resp, nil
}

// --- Audit helper ---

// IssueLoginNonce generates a short-lived nonce for PKI login step 1.
// The caller (api-gateway) must forward this nonce to the client, which signs
// it with its X.509 private key and sends it back via VerifyPKILogin.
func (s *identityService) IssueLoginNonce(ctx context.Context, req *contract.IssueLoginNonceRequest) (*contract.IssueLoginNonceResponse, error) {
	if req.UserID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, status.Errorf(codes.Internal, "generate nonce: %v", err)
	}
	nonce := hex.EncodeToString(raw)

	s.nonceMu.Lock()
	s.nonces[req.UserID] = nonceEntry{nonce: nonce, expiresAt: time.Now().Add(5 * time.Minute)}
	s.nonceMu.Unlock()

	return &contract.IssueLoginNonceResponse{Nonce: nonce}, nil
}

// VerifyPKILogin completes PKI login step 2:
//  1. Verifies the X.509 cert chain against the Central Bank CA.
//  2. Validates the nonce signature against the cert's public key.
//  3. Issues a Keycloak token on success.
func (s *identityService) VerifyPKILogin(ctx context.Context, req *contract.VerifyPKILoginRequest) (*contract.VerifyPKILoginResponse, error) {
	if req.UserID == "" || req.NonceSignatureHex == "" || req.CertPEM == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, nonce_signature_hex, and cert_pem are required")
	}

	// 1. Retrieve and validate nonce
	s.nonceMu.Lock()
	entry, ok := s.nonces[req.UserID]
	if ok {
		delete(s.nonces, req.UserID)
	}
	s.nonceMu.Unlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, status.Error(codes.Unauthenticated, "nonce expired or not found — restart PKI login")
	}

	// 2. Verify certificate chain against Central Bank CA
	if s.caCertPEM != "" {
		if err := pki.VerifyChain(req.CertPEM, s.caCertPEM); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "PKI: %v", err)
		}
	}

	// 3. Validate nonce signature against cert's public key
	if err := pki.ValidateSignature(req.CertPEM, entry.nonce, req.NonceSignatureHex); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "PKI signature: %v", err)
	}

	// 4. Issue Keycloak token using the user's credentials (service-account flow)
	tr, err := s.keycloak.Login(ctx, req.UserID, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: keycloak token: %v", err)
	}

	s.emitAudit(ctx, "PKI_LOGIN", req.UserID, "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")

	return &contract.VerifyPKILoginResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    "Bearer",
	}, nil
}

// emitAudit fires an audit log entry asynchronously (fire-and-forget).
// Failures in audit logging must NOT block the main business operation.
func (s *identityService) emitAudit(ctx context.Context, action, actorSubject, actorAddress, targetSubject, correlationID, ip, result string) {
	go func() {
		if err := s.compliance.CreateAuditLog(context.Background(), complianceclient.AuditEntry{
			ActionType:    action,
			ActorSubject:  actorSubject,
			ActorAddress:  actorAddress,
			TargetSubject: targetSubject,
			CorrelationID: correlationID,
			IPAddress:     ip,
			Result:        result,
			Category:      "SESSION",
			Severity:      "INFO",
		}); err != nil {
			log.Printf("WARN: audit emit failed: %v", err)
		}
	}()
}

// correlationIDFromCtx extracts X-Correlation-Id from gRPC incoming metadata.
func correlationIDFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// ipAddressFromCtx extracts the client IP from gRPC incoming metadata.
func ipAddressFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// isConflict returns true if the error message indicates a duplicate/conflict.
func isConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"conflict", "duplicate", "already exists", "unique"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// --- gRPC handler adapters (boilerplate) ---

func loginHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.LoginMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).Login(ctx, req.(*contract.LoginRequest))
	})
}

func refreshTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RefreshTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RefreshToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RefreshTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RefreshToken(ctx, req.(*contract.RefreshTokenRequest))
	})
}

func revokeTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RevokeTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RevokeToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RevokeTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RevokeToken(ctx, req.(*contract.RevokeTokenRequest))
	})
}

func validateTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ValidateTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ValidateToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ValidateTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ValidateToken(ctx, req.(*contract.ValidateTokenRequest))
	})
}

func registerParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RegisterParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RegisterParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RegisterParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RegisterParticipant(ctx, req.(*contract.RegisterParticipantRequest))
	})
}

func signTransactionHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.SignTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).SignTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.SignTransactionMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).SignTransaction(ctx, req.(*contract.SignTransactionRequest))
	})
}

func getKYCStatusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetKYCStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).GetKYCStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetKYCStatusMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).GetKYCStatus(ctx, req.(*contract.GetKYCStatusRequest))
	})
}

func provisionParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ProvisionParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ProvisionParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ProvisionParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ProvisionParticipant(ctx, req.(*contract.ProvisionParticipantRequest))
	})
}

func onboardParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.OnboardParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).OnboardParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.OnboardParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).OnboardParticipant(ctx, req.(*contract.OnboardParticipantRequest))
	})
}

func issueLoginNonceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.IssueLoginNonceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).IssueLoginNonce(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.IssueLoginNonceMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).IssueLoginNonce(ctx, req.(*contract.IssueLoginNonceRequest))
	})
}

func verifyPKILoginHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.VerifyPKILoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).VerifyPKILogin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.VerifyPKILoginMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).VerifyPKILogin(ctx, req.(*contract.VerifyPKILoginRequest))
	})
}
