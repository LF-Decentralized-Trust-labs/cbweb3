package server

import (
	"context"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type dataAccessService struct {
	participants repository.ParticipantsRepository
}

type serviceInterface interface {
	UpsertParticipant(ctx context.Context, req *contract.UpsertParticipantRequest) (*contract.UpsertParticipantResponse, error)
	GetParticipantByUser(ctx context.Context, req *contract.GetParticipantByUserRequest) (*contract.GetParticipantByUserResponse, error)
}

func New(participants repository.ParticipantsRepository) *grpc.Server {
	svc := &dataAccessService{participants: participants}
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*serviceInterface)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "UpsertParticipant", Handler: upsertParticipantHandler},
			{MethodName: "GetParticipantByUser", Handler: getParticipantByUserHandler},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "dataaccess.v1",
	}, svc)
	return grpcServer
}

func (s *dataAccessService) UpsertParticipant(ctx context.Context, req *contract.UpsertParticipantRequest) (*contract.UpsertParticipantResponse, error) {
	if strings.TrimSpace(req.Participant.UserID) == "" {
		return nil, status.Error(codes.InvalidArgument, "participant.user_id is required")
	}
	err := s.participants.Upsert(ctx, repository.Participant{
		UserID:         req.Participant.UserID,
		DID:            req.Participant.DID,
		WalletAddress:  req.Participant.WalletAddress,
		Country:        req.Participant.Country,
		BankCode:       req.Participant.BankCode,
		Role:           req.Participant.Role,
		SignerProvider: req.Participant.SignerProvider,
		KMSKeyID:       req.Participant.KMSKeyID,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "required") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.UpsertParticipantResponse{Success: true}, nil
}

func (s *dataAccessService) GetParticipantByUser(ctx context.Context, req *contract.GetParticipantByUserRequest) (*contract.GetParticipantByUserResponse, error) {
	if strings.TrimSpace(req.UserID) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	p, found, err := s.participants.GetByUser(ctx, req.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &contract.GetParticipantByUserResponse{Found: false}, nil
	}
	return &contract.GetParticipantByUserResponse{
		Found: true,
		Participant: contract.Participant{
			UserID:         p.UserID,
			DID:            p.DID,
			WalletAddress:  p.WalletAddress,
			Country:        p.Country,
			BankCode:       p.BankCode,
			Role:           p.Role,
			SignerProvider: p.SignerProvider,
			KMSKeyID:       p.KMSKeyID,
		},
	}, nil
}

func upsertParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.UpsertParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*dataAccessService).UpsertParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.UpsertParticipantMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*dataAccessService).UpsertParticipant(ctx, req.(*contract.UpsertParticipantRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func getParticipantByUserHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetParticipantByUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*dataAccessService).GetParticipantByUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetParticipantByUserMethod}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*dataAccessService).GetParticipantByUser(ctx, req.(*contract.GetParticipantByUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}
