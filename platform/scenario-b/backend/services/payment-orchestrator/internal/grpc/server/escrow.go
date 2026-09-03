// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- Deposit lifecycle ---

// RegisterDeposit creates a new fiat deposit request from a commercial bank.
func (s *paymentOrchestratorService) RegisterDeposit(ctx context.Context, req *pb.RegisterDepositRequest) (*pb.RegisterDepositResponse, error) {
	if req.RequesterBesuAddress == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "requester_besu_address and amount are required")
	}

	id := generateID()
	record := domain.DepositRecord{
		ID:                   id,
		RequesterID:          req.RequesterBesuAddress,
		RequesterBesuAddress: req.RequesterBesuAddress,
		Amount:               req.Amount,
		Status:               domain.DepositStatusPending,
		CreatedAt:            time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create deposit: %v", err)
	}

	s.logger.Info("deposit registered", "deposit_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RegisterDepositResponse{DepositId: id}, nil
}

// ApproveDeposit approves a pending deposit and mints fCeBM to the commercial bank.
func (s *paymentOrchestratorService) ApproveDeposit(ctx context.Context, req *pb.ApproveDepositRequest) (*pb.ApproveDepositResponse, error) {
	if req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id is required")
	}

	record, found, err := s.escrowRepo.GetDeposit(ctx, req.DepositId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get deposit: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "deposit not found")
	}
	if record.Status != domain.DepositStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "deposit is %s, not PENDING", record.Status)
	}

	record.Status = domain.DepositStatusApproved
	if err := s.escrowRepo.UpdateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update deposit: %v", err)
	}

	s.logger.Info("deposit approved", "deposit_id", req.DepositId)

	// Mint fCeBM to the commercial bank's Besu address.
	if s.fiat != nil && record.RequesterBesuAddress != "" {
		s.logger.Info("auto-minting fCeBM after approval", "to", record.RequesterBesuAddress, "amount", record.Amount)
		txHash, mintErr := s.fiat.Mint(ctx, record.RequesterBesuAddress, record.Amount)
		if mintErr != nil {
			record.Status = domain.DepositStatusMintFailed
			if updateErr := s.escrowRepo.UpdateDeposit(ctx, record); updateErr != nil {
				s.logger.Error("failed to persist MINT_FAILED status", "deposit_id", req.DepositId, "err", updateErr)
			}
			s.logger.Error("auto-mint fCeBM failed", "deposit_id", req.DepositId, "err", mintErr)
		} else {
			record.FiatMintTxHash = txHash
			if err := s.escrowRepo.UpdateDeposit(ctx, record); err != nil {
				s.logger.Error("failed to persist fiat mint tx hash", "deposit_id", req.DepositId, "tx_hash", txHash, "err", err)
			} else {
				s.logger.Info("fCeBM auto-minted", "deposit_id", req.DepositId, "fiat_mint_tx_hash", txHash)
			}
		}
	} else {
		if s.fiat == nil {
			s.logger.Warn("fCeBM adapter not configured — skipping auto-mint", "deposit_id", req.DepositId)
		}
		if record.RequesterBesuAddress == "" {
			s.logger.Warn("requester Besu address missing — skipping auto-mint", "deposit_id", req.DepositId)
		}
	}

	return &pb.ApproveDepositResponse{FiatMintTxHash: record.FiatMintTxHash}, nil
}

// RejectDeposit rejects a pending deposit request (central bank action).
func (s *paymentOrchestratorService) RejectDeposit(ctx context.Context, req *pb.RejectDepositRequest) (*pb.RejectDepositResponse, error) {
	if req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id is required")
	}

	record, found, err := s.escrowRepo.GetDeposit(ctx, req.DepositId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get deposit: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "deposit not found")
	}
	if record.Status != domain.DepositStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "deposit is %s, not PENDING", record.Status)
	}

	record.Status = domain.DepositStatusRejected
	record.RejectionReason = req.Reason
	if err := s.escrowRepo.UpdateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update deposit: %v", err)
	}

	s.logger.Info("deposit rejected", "deposit_id", req.DepositId, "reason", req.Reason)
	return &pb.RejectDepositResponse{}, nil
}

// RequestFiatExchange retries minting fCeBM for a deposit that failed the initial auto-mint.
func (s *paymentOrchestratorService) RequestFiatExchange(ctx context.Context, req *pb.RequestFiatExchangeRequest) (*pb.RequestFiatExchangeResponse, error) {
	if req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id is required")
	}

	record, found, err := s.escrowRepo.GetDeposit(ctx, req.DepositId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get deposit: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "deposit not found")
	}
	if record.Status != domain.DepositStatusApproved && record.Status != domain.DepositStatusMintFailed {
		return nil, status.Errorf(codes.FailedPrecondition, "deposit is %s, not APPROVED", record.Status)
	}
	if record.FiatMintTxHash != "" {
		return nil, status.Error(codes.AlreadyExists, "fCeBM already minted for this deposit")
	}

	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fCeBM token adapter not configured")
	}

	s.logger.Info("minting fCeBM", "to", record.RequesterBesuAddress, "amount", record.Amount)
	txHash, err := s.fiat.Mint(ctx, record.RequesterBesuAddress, record.Amount)
	if err != nil {
		record.Status = domain.DepositStatusMintFailed
		if updateErr := s.escrowRepo.UpdateDeposit(ctx, record); updateErr != nil {
			s.logger.Error("failed to persist MINT_FAILED status", "deposit_id", req.DepositId, "err", updateErr)
		}
		return nil, status.Errorf(codes.Internal, "fCeBM mint: %v", err)
	}

	record.FiatMintTxHash = txHash
	if err := s.escrowRepo.UpdateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update deposit: %v", err)
	}

	s.logger.Info("fCeBM minted", "deposit_id", req.DepositId, "fiat_mint_tx_hash", txHash)
	return &pb.RequestFiatExchangeResponse{MintTxHash: txHash}, nil
}

// ListDeposits returns all deposit records, optionally filtered by requester.
func (s *paymentOrchestratorService) ListDeposits(ctx context.Context, req *pb.ListDepositsRequest) (*pb.ListDepositsResponse, error) {
	records, err := s.escrowRepo.ListDeposits(ctx, req.RequesterId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deposits: %v", err)
	}

	var pbRecords []*pb.DepositRecord
	for _, r := range records {
		pbRecords = append(pbRecords, &pb.DepositRecord{
			Id:                   r.ID,
			RequesterId:          r.RequesterID,
			RequesterBesuAddress: r.RequesterBesuAddress,
			Amount:               r.Amount,
			Status:               depositStatusToProto(r.Status),
			FiatMintTxHash:       r.FiatMintTxHash,
			RejectionReason:      r.RejectionReason,
			CreatedAt:            r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &pb.ListDepositsResponse{Deposits: pbRecords}, nil
}

// --- Redeem lifecycle ---

// RequestRedeem creates a new redeem (de-tokenization) request from a commercial bank.
func (s *paymentOrchestratorService) RequestRedeem(ctx context.Context, req *pb.RequestRedeemRequest) (*pb.RequestRedeemResponse, error) {
	if req.RequesterBesuAddress == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "requester_besu_address and amount are required")
	}

	id := generateID()
	record := domain.RedeemRecord{
		ID:                   id,
		RequesterID:          req.RequesterBesuAddress,
		RequesterBesuAddress: req.RequesterBesuAddress,
		Amount:               req.Amount,
		Status:               domain.RedeemStatusPending,
		CreatedAt:            time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateRedeem(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create redeem: %v", err)
	}

	s.logger.Info("redeem requested", "redeem_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RequestRedeemResponse{RedeemId: id}, nil
}

// ApproveRedeem approves a redeem (de-tokenization) request: it burns tCeBM from the
// commercial bank's address and mints the equivalent fCeBM (fiat) back to the same
// address. This is the exact inverse of ApproveEscrow (fCeBM → tCeBM); redeeming must
// DECREASE the caller's tCeBM balance, never mint more of it.
func (s *paymentOrchestratorService) ApproveRedeem(ctx context.Context, req *pb.ApproveRedeemRequest) (*pb.ApproveRedeemResponse, error) {
	if req.RedeemId == "" {
		return nil, status.Error(codes.InvalidArgument, "redeem_id is required")
	}

	record, found, err := s.escrowRepo.GetRedeem(ctx, req.RedeemId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get redeem: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "redeem not found")
	}
	if record.Status != domain.RedeemStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "redeem is %s, not PENDING", record.Status)
	}

	if s.token == nil {
		return nil, status.Error(codes.Unavailable, "tCeBM token adapter not configured")
	}
	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fCeBM token adapter not configured")
	}

	// Step 1: burn tCeBM from the commercial bank's address (de-tokenization).
	s.logger.Info("burning tCeBM for redeem", "from", record.RequesterBesuAddress, "amount", record.Amount)
	burnTxHash, burnErr := s.token.Burn(ctx, record.RequesterBesuAddress, record.Amount)
	if burnErr != nil {
		return nil, status.Errorf(codes.Internal, "tCeBM burn: %v", burnErr)
	}

	// Step 2: mint the equivalent fCeBM (fiat) back to the commercial bank's address.
	s.logger.Info("minting fCeBM for redeem", "to", record.RequesterBesuAddress, "amount", record.Amount)
	fiatMintTxHash, mintErr := s.fiat.Mint(ctx, record.RequesterBesuAddress, record.Amount)
	if mintErr != nil {
		return nil, status.Errorf(codes.Internal, "fCeBM mint: %v", mintErr)
	}

	record.Status = domain.RedeemStatusApproved
	record.MintTxHash = fiatMintTxHash
	if err := s.escrowRepo.UpdateRedeem(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update redeem: %v", err)
	}

	s.logger.Info("redeem approved", "redeem_id", req.RedeemId, "tcebm_burn_tx_hash", burnTxHash, "fiat_mint_tx_hash", fiatMintTxHash)
	return &pb.ApproveRedeemResponse{MintTxHash: fiatMintTxHash}, nil
}

// RejectRedeem rejects a pending redeem request.
func (s *paymentOrchestratorService) RejectRedeem(ctx context.Context, req *pb.RejectRedeemRequest) (*pb.RejectRedeemResponse, error) {
	if req.RedeemId == "" {
		return nil, status.Error(codes.InvalidArgument, "redeem_id is required")
	}

	record, found, err := s.escrowRepo.GetRedeem(ctx, req.RedeemId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get redeem: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "redeem not found")
	}
	if record.Status != domain.RedeemStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "redeem is %s, not PENDING", record.Status)
	}

	record.Status = domain.RedeemStatusRejected
	record.RejectionReason = req.Reason
	if err := s.escrowRepo.UpdateRedeem(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update redeem: %v", err)
	}

	s.logger.Info("redeem rejected", "redeem_id", req.RedeemId, "reason", req.Reason)
	return &pb.RejectRedeemResponse{}, nil
}

// ListRedeems returns all redeem records, optionally filtered by requester.
func (s *paymentOrchestratorService) ListRedeems(ctx context.Context, req *pb.ListRedeemsRequest) (*pb.ListRedeemsResponse, error) {
	records, err := s.escrowRepo.ListRedeems(ctx, req.RequesterId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list redeems: %v", err)
	}

	var pbRecords []*pb.RedeemRecord
	for _, r := range records {
		pbRecords = append(pbRecords, &pb.RedeemRecord{
			Id:                   r.ID,
			RequesterId:          r.RequesterID,
			RequesterBesuAddress: r.RequesterBesuAddress,
			Amount:               r.Amount,
			Status:               redeemStatusToProto(r.Status),
			MintTxHash:           r.MintTxHash,
			RejectionReason:      r.RejectionReason,
			CreatedAt:            r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &pb.ListRedeemsResponse{Redeems: pbRecords}, nil
}

// --- Escrow (Tokenization) lifecycle: fCeBM → tCeBM ---

// RequestEscrow creates a tokenization request: the bank wants to convert fCeBM into tCeBM.
func (s *paymentOrchestratorService) RequestEscrow(ctx context.Context, req *pb.RequestEscrowRequest) (*pb.RequestEscrowResponse, error) {
	if req.RequesterBesuAddress == "" || req.Amount == "" || req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "requester_besu_address, amount and deposit_id are required")
	}

	id := generateID()
	record := domain.EscrowRecord{
		ID:          id,
		RequesterID: req.RequesterBesuAddress,
		BesuAddress: req.RequesterBesuAddress,
		DepositID:   req.DepositId,
		Amount:      req.Amount,
		Status:      domain.EscrowStatusPending,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateEscrow(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create escrow: %v", err)
	}

	s.logger.Info("escrow requested", "escrow_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RequestEscrowResponse{EscrowId: id}, nil
}

// ApproveEscrow approves a tokenization request: burns fCeBM and mints tCeBM on-chain.
func (s *paymentOrchestratorService) ApproveEscrow(ctx context.Context, req *pb.ApproveEscrowRequest) (*pb.ApproveEscrowResponse, error) {
	if req.EscrowId == "" {
		return nil, status.Error(codes.InvalidArgument, "escrow_id is required")
	}

	record, found, err := s.escrowRepo.GetEscrow(ctx, req.EscrowId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get escrow: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "escrow not found")
	}
	if record.Status != domain.EscrowStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "escrow is %s, not PENDING", record.Status)
	}

	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fCeBM token adapter not configured")
	}
	if s.token == nil {
		return nil, status.Error(codes.Unavailable, "tCeBM token adapter not configured")
	}

	// Step 1: burn fCeBM from the commercial bank's address.
	s.logger.Info("burning fCeBM for escrow", "from", record.BesuAddress, "amount", record.Amount)
	burnTxHash, burnErr := s.fiat.Burn(ctx, record.BesuAddress, record.Amount)
	if burnErr != nil {
		return nil, status.Errorf(codes.Internal, "fCeBM burn: %v", burnErr)
	}
	record.BurnTxHash = burnTxHash

	// Step 2: mint tCeBM to the commercial bank's address.
	s.logger.Info("minting tCeBM for escrow", "to", record.BesuAddress, "amount", record.Amount)
	mintTxHash, mintErr := s.token.Mint(ctx, record.BesuAddress, record.Amount)
	if mintErr != nil {
		return nil, status.Errorf(codes.Internal, "tCeBM mint: %v", mintErr)
	}
	record.MintTxHash = mintTxHash

	record.Status = domain.EscrowStatusApproved
	if err := s.escrowRepo.UpdateEscrow(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update escrow: %v", err)
	}

	s.logger.Info("escrow approved", "escrow_id", req.EscrowId, "burn_tx_hash", burnTxHash, "mint_tx_hash", mintTxHash)
	return &pb.ApproveEscrowResponse{BurnTxHash: burnTxHash, MintTxHash: mintTxHash}, nil
}

// RejectEscrow rejects a pending tokenization request.
func (s *paymentOrchestratorService) RejectEscrow(ctx context.Context, req *pb.RejectEscrowRequest) (*pb.RejectEscrowResponse, error) {
	if req.EscrowId == "" {
		return nil, status.Error(codes.InvalidArgument, "escrow_id is required")
	}

	record, found, err := s.escrowRepo.GetEscrow(ctx, req.EscrowId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get escrow: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "escrow not found")
	}
	if record.Status != domain.EscrowStatusPending {
		return nil, status.Errorf(codes.FailedPrecondition, "escrow is %s, not PENDING", record.Status)
	}

	record.Status = domain.EscrowStatusRejected
	record.RejectionReason = req.Reason
	if err := s.escrowRepo.UpdateEscrow(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update escrow: %v", err)
	}

	s.logger.Info("escrow rejected", "escrow_id", req.EscrowId, "reason", req.Reason)
	return &pb.RejectEscrowResponse{}, nil
}

// ListEscrows returns all escrow records, optionally filtered by requester.
func (s *paymentOrchestratorService) ListEscrows(ctx context.Context, req *pb.ListEscrowsRequest) (*pb.ListEscrowsResponse, error) {
	records, err := s.escrowRepo.ListEscrows(ctx, req.RequesterId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list escrows: %v", err)
	}

	var pbRecords []*pb.EscrowRecord
	for _, r := range records {
		pbRecords = append(pbRecords, escrowRecordToProto(r))
	}
	return &pb.ListEscrowsResponse{Escrows: pbRecords}, nil
}

// GetFiatBalance returns the fCeBM balance for the calling address or a specified address.
func (s *paymentOrchestratorService) GetFiatBalance(ctx context.Context, req *pb.GetFiatBalanceRequest) (*pb.GetFiatBalanceResponse, error) {
	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fCeBM token adapter not configured")
	}

	var (
		balance string
		err     error
	)
	if req.Address != "" {
		balance, err = s.fiat.BalanceOf(ctx, req.Address)
	} else {
		balance, err = s.fiat.GetFiatBalance(ctx)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fCeBM balance: %v", err)
	}

	decimals, err := s.fiat.Decimals(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fCeBM decimals: %v", err)
	}

	// Symbol is the source of truth for the fiat currency code shown in the UI, but is
	// non-essential to the balance value: a read failure must not fail the call. Clients
	// fall back to their configured fiat symbol.
	symbol, err := s.fiat.Symbol(ctx)
	if err != nil {
		s.logger.Warn("fCeBM symbol read failed; returning balance without symbol", "error", err)
		symbol = ""
	}

	return &pb.GetFiatBalanceResponse{Balance: balance, Decimals: uint32(decimals), Symbol: symbol}, nil
}

func escrowRecordToProto(r domain.EscrowRecord) *pb.EscrowRecord {
	statusMap := map[domain.EscrowStatus]pb.EscrowStatus{
		domain.EscrowStatusPending:  pb.EscrowStatus_ESCROW_STATUS_PENDING,
		domain.EscrowStatusApproved: pb.EscrowStatus_ESCROW_STATUS_APPROVED,
		domain.EscrowStatusRejected: pb.EscrowStatus_ESCROW_STATUS_REJECTED,
	}
	return &pb.EscrowRecord{
		Id:              r.ID,
		RequesterId:     r.RequesterID,
		BesuAddress:     r.BesuAddress,
		DepositId:       r.DepositID,
		Amount:          r.Amount,
		Status:          statusMap[r.Status],
		BurnTxHash:      r.BurnTxHash,
		MintTxHash:      r.MintTxHash,
		RejectionReason: r.RejectionReason,
		CreatedAt:       r.CreatedAt.Format(time.RFC3339),
	}
}

// --- Proto enum converters ---

func depositStatusToProto(s domain.DepositStatus) pb.DepositStatus {
	switch s {
	case domain.DepositStatusApproved:
		return pb.DepositStatus_DEPOSIT_STATUS_APPROVED
	case domain.DepositStatusRejected:
		return pb.DepositStatus_DEPOSIT_STATUS_REJECTED
	case domain.DepositStatusMintFailed:
		return pb.DepositStatus_DEPOSIT_STATUS_MINT_FAILED
	default:
		return pb.DepositStatus_DEPOSIT_STATUS_PENDING
	}
}

func redeemStatusToProto(s domain.RedeemStatus) pb.RedeemStatus {
	switch s {
	case domain.RedeemStatusApproved:
		return pb.RedeemStatus_REDEEM_STATUS_APPROVED
	case domain.RedeemStatusRejected:
		return pb.RedeemStatus_REDEEM_STATUS_REJECTED
	default:
		return pb.RedeemStatus_REDEEM_STATUS_PENDING
	}
}
