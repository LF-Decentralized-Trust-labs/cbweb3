package server

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/data-access/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type participantsRepoStub struct {
	upsertFn func(ctx context.Context, p repository.Participant) error
	getFn    func(ctx context.Context, userID string) (repository.Participant, bool, error)
}

func (s participantsRepoStub) Upsert(ctx context.Context, p repository.Participant) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, p)
	}
	return nil
}

func (s participantsRepoStub) GetByUser(ctx context.Context, userID string) (repository.Participant, bool, error) {
	if s.getFn != nil {
		return s.getFn(ctx, userID)
	}
	return repository.Participant{}, false, nil
}

func TestUpsertParticipantInvalidArgument(t *testing.T) {
	t.Parallel()

	svc := &dataAccessService{
		participants: participantsRepoStub{},
	}
	_, err := svc.UpsertParticipant(context.Background(), &contract.UpsertParticipantRequest{
		Participant: contract.Participant{UserID: ""},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestGetParticipantByUserInvalidArgument(t *testing.T) {
	t.Parallel()

	svc := &dataAccessService{
		participants: participantsRepoStub{},
	}
	_, err := svc.GetParticipantByUser(context.Background(), &contract.GetParticipantByUserRequest{
		UserID: "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestUpsertParticipantInternalError(t *testing.T) {
	t.Parallel()

	svc := &dataAccessService{
		participants: participantsRepoStub{
			upsertFn: func(_ context.Context, _ repository.Participant) error {
				return errors.New("db unavailable")
			},
		},
	}
	_, err := svc.UpsertParticipant(context.Background(), &contract.UpsertParticipantRequest{
		Participant: contract.Participant{UserID: "bank-a"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
}
