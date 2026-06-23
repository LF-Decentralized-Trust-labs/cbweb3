// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const tokenDecimals = 18

func (s *complianceService) CreateTransferLimit(ctx context.Context, req *compliancv1.CreateTransferLimitRequest) (*compliancv1.CreateTransferLimitResponse, error) {
	if strings.TrimSpace(req.MaxAmount) == "" {
		return nil, status.Error(codes.InvalidArgument, "max_amount is required")
	}
	if _, err := humanToWei(req.MaxAmount); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid max_amount: %v", err)
	}

	cbID := centralBankIDFromCtx(ctx)

	limit := repository.TransferLimit{
		LimitID:       uuid.NewString(),
		CentralBankID: cbID,
		ParticipantID: strings.TrimSpace(req.ParticipantId),
		Currency:      strings.TrimSpace(req.Currency),
		MaxAmount:     strings.TrimSpace(req.MaxAmount),
		IsActive:      true,
	}
	if err := s.repo.CreateTransferLimit(ctx, limit); err != nil {
		return nil, status.Errorf(codes.Internal, "create transfer limit: %v", err)
	}

	go s.emitAudit(ctx, "CREATE_TRANSFER_LIMIT", req.ActorSubject, "", limit.ParticipantID,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS", "TRANSFER_LIMIT", "INFO",
		fmt.Sprintf(`{"limit_id":%q,"max_amount":%q}`, limit.LimitID, limit.MaxAmount))

	return &compliancv1.CreateTransferLimitResponse{Limit: transferLimitToProto(limit)}, nil
}

func (s *complianceService) ListTransferLimits(ctx context.Context, req *compliancv1.ListTransferLimitsRequest) (*compliancv1.ListTransferLimitsResponse, error) {
	cbID := req.CentralBankId
	if cbID == "" {
		cbID = centralBankIDFromCtx(ctx)
	}
	limits, err := s.repo.ListTransferLimits(ctx, cbID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list transfer limits: %v", err)
	}
	pb := make([]*compliancv1.TransferLimit, len(limits))
	for i, l := range limits {
		pb[i] = transferLimitToProto(l)
	}
	return &compliancv1.ListTransferLimitsResponse{Limits: pb}, nil
}

func (s *complianceService) DeleteTransferLimit(ctx context.Context, req *compliancv1.DeleteTransferLimitRequest) (*emptypb.Empty, error) {
	if req.LimitId == "" {
		return nil, status.Error(codes.InvalidArgument, "limit_id is required")
	}
	if err := s.repo.DeleteTransferLimit(ctx, req.LimitId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete transfer limit: %v", err)
	}
	go s.emitAudit(ctx, "DELETE_TRANSFER_LIMIT", req.ActorSubject, "", req.LimitId,
		correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS", "TRANSFER_LIMIT", "INFO",
		fmt.Sprintf(`{"limit_id":%q}`, req.LimitId))
	return &emptypb.Empty{}, nil
}

func (s *complianceService) CheckAndDeductTransferLimit(ctx context.Context, req *compliancv1.CheckAndDeductTransferLimitRequest) (*compliancv1.CheckAndDeductTransferLimitResponse, error) {
	if req.PayerBankId == "" || req.AmountHuman == "" {
		return nil, status.Error(codes.InvalidArgument, "payer_bank_id and amount_human are required")
	}

	cbID := centralBankIDForPayer(req.PayerBankId)
	if cbID == "" {
		return &compliancv1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
	}

	amountWei, err := humanToWei(req.AmountHuman)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount_human: %v", err)
	}

	limit, err := s.repo.FindApplicableLimit(ctx, cbID, req.PayerBankId, req.Currency)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "limit lookup: %v", err)
	}
	if limit == nil {
		return &compliancv1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
	}

	maxWei, err := humanToWei(limit.MaxAmount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "malformed limit max_amount: %v", err)
	}

	today := utcDay(time.Now().UTC())
	accStr, err := s.repo.GetAccumulatedVolume(ctx, req.PayerBankId, req.Currency, today)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume lookup: %v", err)
	}
	accWei, _ := new(big.Int).SetString(accStr, 10)
	if accWei == nil {
		accWei = new(big.Int)
	}

	if new(big.Int).Add(accWei, amountWei).Cmp(maxWei) > 0 {
		return &compliancv1.CheckAndDeductTransferLimitResponse{
			Allowed:   false,
			ErrorCode: "TRANSFER_LIMIT_EXCEEDED",
			MaxAmount: limit.MaxAmount,
		}, nil
	}

	if err := s.repo.DeductTransferVolume(ctx, req.PayerBankId, req.Currency, amountWei.String(), today); err != nil {
		return nil, status.Errorf(codes.Internal, "volume deduct: %v", err)
	}
	return &compliancv1.CheckAndDeductTransferLimitResponse{Allowed: true}, nil
}

func (s *complianceService) RestoreTransferLimit(ctx context.Context, req *compliancv1.RestoreTransferLimitRequest) (*emptypb.Empty, error) {
	if req.PayerBankId == "" || req.AmountHuman == "" {
		return &emptypb.Empty{}, nil
	}
	amountWei, err := humanToWei(req.AmountHuman)
	if err != nil {
		return &emptypb.Empty{}, nil
	}
	today := utcDay(time.Now().UTC())
	_ = s.repo.RestoreTransferVolume(ctx, req.PayerBankId, req.Currency, amountWei.String(), today)
	return &emptypb.Empty{}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func centralBankIDFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md["x-bank-code"]; len(vals) > 0 {
			return centralBankIDForPayer(vals[0])
		}
	}
	return "central-bank-a"
}

func centralBankIDForPayer(payerBankID string) string {
	lower := strings.ToLower(strings.TrimSpace(payerBankID))
	if strings.HasSuffix(lower, "-a") {
		return "central-bank-a"
	}
	if strings.HasSuffix(lower, "-b") {
		return "central-bank-b"
	}
	return ""
}

func humanToWei(human string) (*big.Int, error) {
	human = strings.TrimSpace(human)
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)
	parts := strings.SplitN(human, ".", 2)
	intPart, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer part %q", parts[0])
	}
	result := new(big.Int).Mul(intPart, multiplier)
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > tokenDecimals {
			frac = frac[:tokenDecimals]
		} else {
			frac = frac + strings.Repeat("0", tokenDecimals-len(frac))
		}
		fracInt, ok := new(big.Int).SetString(frac, 10)
		if !ok {
			return nil, fmt.Errorf("invalid fractional part %q", frac)
		}
		result.Add(result, fracInt)
	}
	return result, nil
}

func utcDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func transferLimitToProto(l repository.TransferLimit) *compliancv1.TransferLimit {
	return &compliancv1.TransferLimit{
		LimitId:       l.LimitID,
		CentralBankId: l.CentralBankID,
		ParticipantId: l.ParticipantID,
		Currency:      l.Currency,
		MaxAmount:     l.MaxAmount,
		IsActive:      l.IsActive,
		CreatedAt:     l.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     l.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
