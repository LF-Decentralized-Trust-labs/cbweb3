package server

import (
	"context"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/dataaccessclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/tokenissuer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type identityService struct {
	provider      identityprovider.Provider
	dataAccess    dataaccessclient.Client
	internalToken tokenissuer.Issuer
}

type identityServiceServer interface {
	Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error)
	RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error)
	RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error)
	CreateWallet(ctx context.Context, req *contract.CreateWalletRequest) (*contract.CreateWalletResponse, error)
	ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error)
	BindWallet(ctx context.Context, req *contract.BindWalletRequest) (*contract.BindWalletResponse, error)
	GetByUser(ctx context.Context, req *contract.GetByUserRequest) (*contract.GetByUserResponse, error)
	RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error)
	IssueKYCCredential(ctx context.Context, req *contract.IssueKYCCredentialRequest) (*contract.IssueKYCCredentialResponse, error)
	VerifyKYCProof(ctx context.Context, req *contract.VerifyKYCProofRequest) (*contract.VerifyKYCProofResponse, error)
	GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error)
	ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error)
}

// New builds a configured gRPC server and registers all identity handlers.
// The KMS provider has been removed — signing is now delegated to the
// identityprovider.Provider (D-Wallet API in prod, LocalProvider in dev).
func New(
	provider identityprovider.Provider,
	dataAccess dataaccessclient.Client,
	internalToken tokenissuer.Issuer,
) *grpc.Server {
	svc := &identityService{
		provider:      provider,
		dataAccess:    dataAccess,
		internalToken: internalToken,
	}
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*identityServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: loginHandler},
			{MethodName: "RefreshToken", Handler: refreshTokenHandler},
			{MethodName: "RevokeToken", Handler: revokeTokenHandler},
			{MethodName: "CreateWallet", Handler: createWalletHandler},
			{MethodName: "ValidateToken", Handler: validateTokenHandler},
			{MethodName: "BindWallet", Handler: bindWalletHandler},
			{MethodName: "GetByUser", Handler: getByUserHandler},
			{MethodName: "RegisterParticipant", Handler: registerParticipantHandler},
			{MethodName: "SignTransaction", Handler: signTransactionHandler},
			{MethodName: "IssueKYCCredential", Handler: issueKYCCredentialHandler},
			{MethodName: "VerifyKYCProof", Handler: verifyKYCProofHandler},
			{MethodName: "GetKYCStatus", Handler: getKYCStatusHandler},
			{MethodName: "ProvisionParticipant", Handler: provisionParticipantHandler},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "identity.v1",
	}, svc)
	return grpcServer
}

// --- Auth handlers ---

func (s *identityService) Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error) {
	out, err := s.provider.Login(ctx, identityprovider.LoginRequest{
		User:     req.User,
		Password: req.Password,
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	claims, err := s.provider.ValidateToken(ctx, out.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// Enrich with participant data from data-access if already registered
	issueReq := tokenissuer.IssueRequest{
		Subject:      claims.Subject,
		Roles:        claims.Roles,
		DID:          claims.DID,
		Wallet:       claims.Wallet,
		Country:      claims.Country,
		BankID:       claims.BankID,
		PrivacyGroup: claims.PrivacyGroup,
	}
	if claims.DID == "" || claims.Wallet == "" {
		if p, found, daErr := s.dataAccess.GetParticipantByUser(ctx, claims.Subject); daErr == nil && found {
			if issueReq.DID == "" {
				issueReq.DID = p.DID
			}
			if issueReq.Wallet == "" {
				issueReq.Wallet = p.WalletAddress
			}
			if issueReq.Country == "" {
				issueReq.Country = p.Country
			}
			if issueReq.BankID == "" {
				issueReq.BankID = p.BankCode
			}
		}
	}

	internalToken, err := s.internalToken.Issue(ctx, issueReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &contract.LoginResponse{
		AccessToken:  internalToken.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    internalToken.TokenType,
		ExpiresIn:    int32(internalToken.ExpiresIn),
	}
	s.emitAudit(ctx, dataaccessclient.AuditEntry{
		ActorSubject: claims.Subject,
		ActionType:   "LOGIN",
		Result:       "SUCCESS",
	})
	return resp, nil
}

func (s *identityService) RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}
	out, err := s.provider.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	claims, err := s.provider.ValidateToken(ctx, out.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	internalToken, err := s.internalToken.Issue(ctx, tokenissuer.IssueRequest{
		Subject: claims.Subject,
		Roles:   claims.Roles,
		DID:     claims.DID,
		Wallet:  claims.Wallet,
		Country: claims.Country,
		BankID:  claims.BankID,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.RefreshTokenResponse{
		AccessToken:  internalToken.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    internalToken.TokenType,
		ExpiresIn:    int32(internalToken.ExpiresIn),
	}, nil
}

func (s *identityService) RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error) {
	if strings.TrimSpace(req.AccessToken) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}
	if err := s.provider.RevokeToken(ctx, req.AccessToken); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.RevokeTokenResponse{Success: true}, nil
}

func (s *identityService) ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error) {
	// Try internal token first
	internalClaims, err := s.internalToken.Validate(ctx, req.AccessToken)
	if err == nil {
		return &contract.ValidateTokenResponse{
			Subject:      internalClaims.Subject,
			Issuer:       s.internalToken.Name(),
			Roles:        internalClaims.Roles,
			DID:          internalClaims.DID,
			Wallet:       internalClaims.Wallet,
			Country:      internalClaims.Country,
			BankID:       internalClaims.BankID,
			PrivacyGroup: internalClaims.PrivacyGroup,
		}, nil
	}
	// Fallback: accept provider token (legacy compatibility during migration)
	legacyClaims, legacyErr := s.provider.ValidateToken(ctx, req.AccessToken)
	if legacyErr != nil {
		return nil, status.Error(codes.Unauthenticated, legacyErr.Error())
	}
	return &contract.ValidateTokenResponse{
		Subject:      legacyClaims.Subject,
		Issuer:       legacyClaims.Issuer,
		Roles:        legacyClaims.Roles,
		DID:          legacyClaims.DID,
		Wallet:       legacyClaims.Wallet,
		Country:      legacyClaims.Country,
		BankID:       legacyClaims.BankID,
		PrivacyGroup: legacyClaims.PrivacyGroup,
	}, nil
}

// --- Wallet handlers ---

func (s *identityService) CreateWallet(ctx context.Context, req *contract.CreateWalletRequest) (*contract.CreateWalletResponse, error) {
	out, err := s.provider.CreateWallet(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.CreateWalletResponse{
		DID:       out.DID,
		Address:   out.Address,
		CreatedAt: out.CreatedAt,
	}, nil
}

func (s *identityService) BindWallet(ctx context.Context, req *contract.BindWalletRequest) (*contract.BindWalletResponse, error) {
	out, err := s.provider.BindWallet(ctx, req.UserID, req.WalletAddress)
	if err != nil {
		switch {
		case strings.Contains(strings.ToLower(err.Error()), "wallet already bound"):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case strings.Contains(strings.ToLower(err.Error()), "user already bound"):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &contract.BindWalletResponse{
		UserID:        out.UserID,
		WalletAddress: out.WalletAddress,
	}, nil
}

func (s *identityService) GetByUser(ctx context.Context, req *contract.GetByUserRequest) (*contract.GetByUserResponse, error) {
	// For DWalletAPIProvider, binding is persisted in data-access
	p, found, err := s.dataAccess.GetParticipantByUser(ctx, req.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if found && p.WalletAddress != "" {
		return &contract.GetByUserResponse{
			Found: true,
			Binding: &contract.BindWalletResponse{
				UserID:        p.UserID,
				WalletAddress: p.WalletAddress,
			},
		}, nil
	}
	// Fallback to provider for LocalProvider (in-memory)
	out, provFound, provErr := s.provider.GetByUser(ctx, req.UserID)
	if provErr != nil {
		return nil, status.Error(codes.Internal, provErr.Error())
	}
	if !provFound {
		return &contract.GetByUserResponse{Found: false}, nil
	}
	return &contract.GetByUserResponse{
		Found: true,
		Binding: &contract.BindWalletResponse{
			UserID:        out.UserID,
			WalletAddress: out.WalletAddress,
		},
	}, nil
}

// --- Participant Registration ---

func (s *identityService) RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error) {
	if strings.TrimSpace(req.AccessToken) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}
	claims, err := s.provider.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	wallet, err := s.provider.CreateWallet(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	binding, err := s.provider.BindWallet(ctx, claims.Subject, wallet.Address)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already bound") {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := s.dataAccess.UpsertParticipant(ctx, dataaccessclient.Participant{
		UserID:          claims.Subject,
		DID:             wallet.DID,
		WalletAddress:   binding.WalletAddress,
		Country:         req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		WalletType:      req.WalletType,
		SignerProvider:  s.provider.Name(),
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &contract.RegisterParticipantResponse{
		UserID:         claims.Subject,
		DID:            wallet.DID,
		WalletAddress:  binding.WalletAddress,
		SignerProvider: s.provider.Name(),
	}
	s.emitAudit(ctx, dataaccessclient.AuditEntry{
		ActorSubject: claims.Subject,
		ActionType:   "REGISTER",
		Result:       "SUCCESS",
	})
	return resp, nil
}

// --- Signing ---

// SignTransaction delegates to the provider — D-Wallet API in prod,
// secp256k1 in-process for LocalProvider. KMS is no longer a separate layer.
func (s *identityService) SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error) {
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.Digest) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and digest are required")
	}
	out, err := s.provider.SignTransaction(ctx, identityprovider.SignRequest{
		UserID:    req.UserID,
		DigestHex: req.Digest,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.SignTransactionResponse{
		UserID:         req.UserID,
		SignerProvider: out.SignerProvider,
		Address:        out.Address,
		Signature:      out.Signature,
	}, nil
}

// --- KYC / Credentials ---

func (s *identityService) IssueKYCCredential(ctx context.Context, req *contract.IssueKYCCredentialRequest) (*contract.IssueKYCCredentialResponse, error) {
	if req.Subject == "" || req.IssuerSubject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and issuer_subject are required")
	}
	out, err := s.provider.IssueKYCCredential(ctx, identityprovider.KYCCredentialRequest{
		Subject:         req.Subject,
		IssuerSubject:   req.IssuerSubject,
		InstitutionName: req.InstitutionName,
		CountryCode:     req.CountryCode,
		BankCode:        req.BankCode,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Persist the KYC credential hash in data-access
	_ = s.dataAccess.UpsertKYCCredential(ctx, dataaccessclient.KYCCredential{
		Subject:    req.Subject,
		ZKPPointer: out.ZKPPointer,
		VCJWT:      out.VCJWT,
		IssuedAt:   out.IssuedAt,
	})
	resp := &contract.IssueKYCCredentialResponse{
		VCJWT:      out.VCJWT,
		ZKPPointer: out.ZKPPointer,
		IssuedAt:   out.IssuedAt,
	}
	s.emitAudit(ctx, dataaccessclient.AuditEntry{
		ActorSubject:  req.IssuerSubject,
		ActionType:    "ISSUE_KYC",
		TargetSubject: req.Subject,
		Result:        "SUCCESS",
	})
	return resp, nil
}

func (s *identityService) VerifyKYCProof(ctx context.Context, req *contract.VerifyKYCProofRequest) (*contract.VerifyKYCProofResponse, error) {
	if req.ZKPPointer == "" {
		return nil, status.Error(codes.InvalidArgument, "zkp_pointer is required")
	}
	valid, err := s.provider.VerifyKYCProof(ctx, req.ZKPPointer)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.VerifyKYCProofResponse{Valid: valid}, nil
}

func (s *identityService) GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error) {
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}
	kycStatus, err := s.provider.GetKYCStatus(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.GetKYCStatusResponse{
		Subject: req.Subject,
		Status:  string(kycStatus),
	}, nil
}

func (s *identityService) ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error) {
	if req.Subject == "" || req.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "subject and status are required")
	}
	if err := s.provider.ProvisionParticipant(ctx, req.Subject, identityprovider.KYCStatus(req.Status)); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	action := "PROVISION"
	switch req.Status {
	case "FROZEN":
		action = "FREEZE"
	case "APPROVED":
		action = "APPROVE"
	case "REVOKED":
		action = "REVOKE"
	}
	s.emitAudit(ctx, dataaccessclient.AuditEntry{
		ActionType:    action,
		TargetSubject: req.Subject,
		Result:        "SUCCESS",
	})
	return &contract.ProvisionParticipantResponse{
		Subject: req.Subject,
		Status:  req.Status,
	}, nil
}

// --- Audit helper ---

// emitAudit fires an audit log entry asynchronously (fire-and-forget).
// Failures in audit logging must NOT block the main business operation.
func (s *identityService) emitAudit(ctx context.Context, entry dataaccessclient.AuditEntry) {
	go func() {
		_ = s.dataAccess.CreateAuditLog(ctx, entry)
	}()
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

func createWalletHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.CreateWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).CreateWallet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.CreateWalletMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).CreateWallet(ctx, req.(*contract.CreateWalletRequest))
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

func bindWalletHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.BindWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).BindWallet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.BindWalletMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).BindWallet(ctx, req.(*contract.BindWalletRequest))
	})
}

func getByUserHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetByUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).GetByUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetByUserMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).GetByUser(ctx, req.(*contract.GetByUserRequest))
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

func issueKYCCredentialHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.IssueKYCCredentialRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).IssueKYCCredential(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.IssueKYCCredentialMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).IssueKYCCredential(ctx, req.(*contract.IssueKYCCredentialRequest))
	})
}

func verifyKYCProofHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.VerifyKYCProofRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).VerifyKYCProof(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.VerifyKYCProofMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).VerifyKYCProof(ctx, req.(*contract.VerifyKYCProofRequest))
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
