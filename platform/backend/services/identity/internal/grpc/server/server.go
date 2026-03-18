package server

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/blockchain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/dataaccessclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kms"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type identityService struct {
	keycloak         keycloak.Client
	kms              kms.Provider
	dataAccess       dataaccessclient.Client
	blockchainClient blockchain.Client
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
}

// New builds a configured gRPC server and registers all identity handlers.
func New(kc keycloak.Client, kmsProvider kms.Provider, da dataaccessclient.Client, bc blockchain.Client) *grpc.Server {
	svc := &identityService{
		keycloak:         kc,
		kms:              kmsProvider,
		dataAccess:       da,
		blockchainClient: bc,
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
		DID:          claims.DID,
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

	upsertErr := s.dataAccess.UpsertParticipant(ctx, dataaccessclient.Participant{
		UserID:          claims.Subject,
		DID:             keyInfo.DID,
		WalletAddress:   keyInfo.Address,
		Country:         req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
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
		DID:           keyInfo.DID,
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
	participant, found, err := s.dataAccess.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &contract.GetKYCStatusResponse{
			Subject: req.Subject,
			Status:  string(domain.KYCStatusPending),
		}, nil
	}
	kycStatus := participant.KYCStatus
	if kycStatus == "" {
		kycStatus = string(domain.KYCStatusPending)
	}
	return &contract.GetKYCStatusResponse{
		Subject: req.Subject,
		Status:  kycStatus,
	}, nil
}

func (s *identityService) ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error) {
	if err := s.dataAccess.UpsertParticipant(ctx, dataaccessclient.Participant{
		UserID:    req.Subject,
		KYCStatus: req.Status,
	}); err != nil {
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

	// 5. Persist participant record.
	if upsertErr := s.dataAccess.UpsertParticipant(ctx, dataaccessclient.Participant{
		UserID:          userID,
		DID:             resp.DID,
		WalletAddress:   resp.WalletAddress,
		Country:         req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
	}); upsertErr != nil {
		return nil, status.Errorf(codes.Internal, "onboard: persisting participant: %v", upsertErr)
	}

	s.emitAudit(ctx, "ONBOARD_PARTICIPANT", userID, resp.WalletAddress, "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return resp, nil
}

// --- Audit helper ---

// emitAudit fires an audit log entry asynchronously (fire-and-forget).
// Failures in audit logging must NOT block the main business operation.
func (s *identityService) emitAudit(ctx context.Context, action, actorSubject, actorAddress, targetSubject, correlationID, ip, result string) {
	go func() {
		if err := s.dataAccess.CreateAuditLog(ctx, dataaccessclient.AuditEntry{
			ActionType:    action,
			ActorSubject:  actorSubject,
			ActorAddress:  actorAddress,
			TargetSubject: targetSubject,
			CorrelationID: correlationID,
			IPAddress:     ip,
			Result:        result,
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
