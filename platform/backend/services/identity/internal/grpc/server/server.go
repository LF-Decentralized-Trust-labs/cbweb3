package server

import (
	"context"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/dataaccessclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/kmsprovider"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/tokenissuer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type identityService struct {
	provider      identityprovider.Provider
	dataAccess    dataaccessclient.Client
	kms           kmsprovider.Provider
	internalToken tokenissuer.Issuer
}

type identityServiceServer interface {
	Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error)
	CreateWallet(ctx context.Context, req *contract.CreateWalletRequest) (*contract.CreateWalletResponse, error)
	ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error)
	BindWallet(ctx context.Context, req *contract.BindWalletRequest) (*contract.BindWalletResponse, error)
	GetByUser(ctx context.Context, req *contract.GetByUserRequest) (*contract.GetByUserResponse, error)
	RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error)
}

// New builds a configured gRPC server and registers identity handlers.
func New(
	provider identityprovider.Provider,
	dataAccess dataaccessclient.Client,
	kms kmsprovider.Provider,
	internalToken tokenissuer.Issuer,
) *grpc.Server {
	svc := &identityService{
		provider:      provider,
		dataAccess:    dataAccess,
		kms:           kms,
		internalToken: internalToken,
	}
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*identityServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Login",
				Handler:    loginHandler,
			},
			{
				MethodName: "CreateWallet",
				Handler:    createWalletHandler,
			},
			{
				MethodName: "ValidateToken",
				Handler:    validateTokenHandler,
			},
			{
				MethodName: "BindWallet",
				Handler:    bindWalletHandler,
			},
			{
				MethodName: "GetByUser",
				Handler:    getByUserHandler,
			},
			{
				MethodName: "RegisterParticipant",
				Handler:    registerParticipantHandler,
			},
			{
				MethodName: "SignTransaction",
				Handler:    signTransactionHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "identity.v1",
	}, svc)
	return grpcServer
}

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
	internalToken, err := s.internalToken.Issue(ctx, tokenissuer.IssueRequest{
		Subject: claims.Subject,
		Roles:   claims.Roles,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &contract.LoginResponse{
		AccessToken:  internalToken.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    internalToken.TokenType,
		ExpiresIn:    int32(internalToken.ExpiresIn),
	}, nil
}

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

func (s *identityService) ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error) {
	out, err := s.internalToken.Validate(ctx, req.AccessToken)
	if err == nil {
		return &contract.ValidateTokenResponse{
			Subject: out.Subject,
			Issuer:  s.internalToken.Name(),
			Roles:   out.Roles,
		}, nil
	}

	legacyClaims, legacyErr := s.provider.ValidateToken(ctx, req.AccessToken)
	if legacyErr != nil {
		return nil, status.Error(codes.Unauthenticated, legacyErr.Error())
	}
	return &contract.ValidateTokenResponse{
		Subject: legacyClaims.Subject,
		Issuer:  legacyClaims.Issuer,
		Roles:   legacyClaims.Roles,
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
	out, found, err := s.provider.GetByUser(ctx, req.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
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
		if strings.Contains(strings.ToLower(err.Error()), "wallet already bound") {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	keyInfo, err := s.kms.CreateKey(ctx, claims.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = s.dataAccess.UpsertParticipant(ctx, dataaccessclient.Participant{
		UserID:         claims.Subject,
		DID:            wallet.DID,
		WalletAddress:  binding.WalletAddress,
		Country:        req.Country,
		BankCode:       req.BankCode,
		Role:           req.Role,
		SignerProvider: s.kms.Name(),
		KMSKeyID:       keyInfo.KeyID,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &contract.RegisterParticipantResponse{
		UserID:         claims.Subject,
		DID:            wallet.DID,
		WalletAddress:  binding.WalletAddress,
		SignerProvider: s.kms.Name(),
		KMSKeyID:       keyInfo.KeyID,
	}, nil
}

func (s *identityService) SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error) {
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.Digest) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and digest are required")
	}
	p, found, err := s.dataAccess.GetParticipantByUser(ctx, req.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "participant not found")
	}
	if strings.TrimSpace(p.KMSKeyID) == "" {
		return nil, status.Error(codes.FailedPrecondition, "participant has no kms key")
	}

	signature, err := s.kms.SignDigest(ctx, p.KMSKeyID, req.Digest)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	keyInfo, ok := s.kms.GetByUser(ctx, req.UserID)
	if !ok {
		return nil, status.Error(codes.Internal, "kms key lookup failed")
	}
	return &contract.SignTransactionResponse{
		UserID:         req.UserID,
		SignerProvider: s.kms.Name(),
		KMSKeyID:       p.KMSKeyID,
		Address:        keyInfo.Address,
		Signature:      signature,
	}, nil
}

func loginHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.LoginMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).Login(ctx, req.(*contract.LoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func createWalletHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.CreateWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).CreateWallet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.CreateWalletMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).CreateWallet(ctx, req.(*contract.CreateWalletRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func validateTokenHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.ValidateTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ValidateToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ValidateTokenMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ValidateToken(ctx, req.(*contract.ValidateTokenRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func bindWalletHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.BindWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).BindWallet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.BindWalletMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).BindWallet(ctx, req.(*contract.BindWalletRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func getByUserHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.GetByUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).GetByUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetByUserMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).GetByUser(ctx, req.(*contract.GetByUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func registerParticipantHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.RegisterParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RegisterParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RegisterParticipantMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RegisterParticipant(ctx, req.(*contract.RegisterParticipantRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func signTransactionHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	in := new(contract.SignTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).SignTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.SignTransactionMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).SignTransaction(ctx, req.(*contract.SignTransactionRequest))
	}
	return interceptor(ctx, in, info, handler)
}
