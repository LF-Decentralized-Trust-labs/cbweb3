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
		ID:                       id,
		RequesterID:              req.RequesterBesuAddress,
		RequesterBesuAddress:     req.RequesterBesuAddress,
		RequesterPaladinIdentity: req.RequesterPaladinIdentity,
		Amount:                   req.Amount,
		Status:                   domain.DepositStatusPending,
		CreatedAt:                time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create deposit: %v", err)
	}

	s.logger.Info("deposit registered", "deposit_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RegisterDepositResponse{DepositId: id}, nil
}

// ApproveDeposit approves a pending deposit request (central bank action).
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
	return &pb.ApproveDepositResponse{}, nil
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

// RequestFiatExchange mints fCeBM to the commercial bank after deposit approval.
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
	if record.MintTxHash != "" {
		return nil, status.Error(codes.AlreadyExists, "fiat exchange already executed for this deposit")
	}

	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fiat token adapter not configured")
	}

	s.logger.Info("minting fCeBM", "to", record.RequesterBesuAddress, "amount", record.Amount)
	txHash, err := s.fiat.Mint(ctx, record.RequesterBesuAddress, record.Amount)
	if err != nil {
		record.Status = domain.DepositStatusMintFailed
		if updateErr := s.escrowRepo.UpdateDeposit(ctx, record); updateErr != nil {
			s.logger.Error("failed to persist MINT_FAILED status", "deposit_id", req.DepositId, "err", updateErr)
		}
		return nil, status.Errorf(codes.Internal, "fiat mint: %v", err)
	}

	record.MintTxHash = txHash
	if err := s.escrowRepo.UpdateDeposit(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update deposit: %v", err)
	}

	s.logger.Info("fiat exchange completed", "deposit_id", req.DepositId, "mint_tx_hash", txHash)
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
			Id:                       r.ID,
			RequesterId:              r.RequesterID,
			RequesterBesuAddress:     r.RequesterBesuAddress,
			RequesterPaladinIdentity: r.RequesterPaladinIdentity,
			Amount:                   r.Amount,
			Status:                   depositStatusToProto(r.Status),
			MintTxHash:               r.MintTxHash,
			RejectionReason:          r.RejectionReason,
			CreatedAt:                r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &pb.ListDepositsResponse{Deposits: pbRecords}, nil
}

// --- Escrow lifecycle ---

// RequestEscrow creates a new escrow (tokenization) request from a commercial bank.
func (s *paymentOrchestratorService) RequestEscrow(ctx context.Context, req *pb.RequestEscrowRequest) (*pb.RequestEscrowResponse, error) {
	if req.RequesterBesuAddress == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "requester_besu_address and amount are required")
	}

	id := generateID()
	record := domain.EscrowRecord{
		ID:                       id,
		RequesterID:              req.RequesterBesuAddress,
		RequesterBesuAddress:     req.RequesterBesuAddress,
		RequesterPaladinIdentity: req.RequesterPaladinIdentity,
		Amount:                   req.Amount,
		Status:                   domain.EscrowStatusPending,
		CreatedAt:                time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateEscrow(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create escrow: %v", err)
	}

	s.logger.Info("escrow requested", "escrow_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RequestEscrowResponse{EscrowId: id}, nil
}

// ApproveEscrow burns fCeBM on Besu and mints tCeBM via Zeto/Paladin.
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
		return nil, status.Error(codes.Unavailable, "fiat token adapter not configured")
	}

	// Step 1: Burn fCeBM from the commercial bank's Besu wallet.
	s.logger.Info("burning fCeBM for escrow", "from", record.RequesterBesuAddress, "amount", record.Amount)
	burnTxHash, err := s.fiat.Burn(ctx, record.RequesterBesuAddress, record.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fiat burn: %v", err)
	}

	// Step 2: Mint tCeBM to the commercial bank's Paladin identity via Zeto.
	s.logger.Info("minting tCeBM via Zeto", "to", record.RequesterPaladinIdentity, "amount", record.Amount)
	mintTxHash, err := s.zeto.Mint(ctx, record.RequesterPaladinIdentity, record.Amount)
	if err != nil {
		// fCeBM was already burned — log critical error. In production, a compensation
		// mechanism should re-mint the burned fCeBM back to the commercial bank.
		s.logger.Error("CRITICAL: fCeBM burned but Zeto mint failed — manual intervention required",
			"escrow_id", req.EscrowId, "burn_tx_hash", burnTxHash, "error", err)
		return nil, status.Errorf(codes.Internal, "zeto mint after burn: %v", err)
	}

	record.Status = domain.EscrowStatusApproved
	record.BurnTxHash = burnTxHash
	record.MintTxHash = mintTxHash
	if err := s.escrowRepo.UpdateEscrow(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update escrow: %v", err)
	}

	s.logger.Info("escrow approved", "escrow_id", req.EscrowId, "burn_tx", burnTxHash, "mint_tx", mintTxHash)
	return &pb.ApproveEscrowResponse{BurnTxHash: burnTxHash, MintTxHash: mintTxHash}, nil
}

// RejectEscrow rejects a pending escrow request (central bank action).
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
		pbRecords = append(pbRecords, &pb.EscrowRecord{
			Id:                       r.ID,
			RequesterId:              r.RequesterID,
			RequesterBesuAddress:     r.RequesterBesuAddress,
			RequesterPaladinIdentity: r.RequesterPaladinIdentity,
			Amount:                   r.Amount,
			Status:                   escrowStatusToProto(r.Status),
			BurnTxHash:               r.BurnTxHash,
			MintTxHash:               r.MintTxHash,
			RejectionReason:          r.RejectionReason,
			CreatedAt:                r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &pb.ListEscrowsResponse{Escrows: pbRecords}, nil
}

// --- Redeem lifecycle ---

// RequestRedeem creates a new redeem (de-tokenization) request from a commercial bank.
func (s *paymentOrchestratorService) RequestRedeem(ctx context.Context, req *pb.RequestRedeemRequest) (*pb.RequestRedeemResponse, error) {
	if req.RequesterBesuAddress == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "requester_besu_address and amount are required")
	}

	id := generateID()
	record := domain.RedeemRecord{
		ID:                       id,
		RequesterID:              req.RequesterBesuAddress,
		RequesterBesuAddress:     req.RequesterBesuAddress,
		RequesterPaladinIdentity: req.RequesterPaladinIdentity,
		Amount:                   req.Amount,
		Status:                   domain.RedeemStatusPending,
		ZetoTransferTxHash:       req.ZetoTransferTxHash,
		CreatedAt:                time.Now().UTC(),
	}

	if err := s.escrowRepo.CreateRedeem(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "create redeem: %v", err)
	}

	s.logger.Info("redeem requested", "redeem_id", id, "requester", req.RequesterBesuAddress, "amount", req.Amount)
	return &pb.RequestRedeemResponse{RedeemId: id}, nil
}

// ApproveRedeem mints fCeBM to the commercial bank on Besu (de-tokenization).
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

	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fiat token adapter not configured")
	}

	// Mint fCeBM back to the commercial bank's Besu address.
	s.logger.Info("minting fCeBM for redeem", "to", record.RequesterBesuAddress, "amount", record.Amount)
	txHash, err := s.fiat.Mint(ctx, record.RequesterBesuAddress, record.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fiat mint: %v", err)
	}

	record.Status = domain.RedeemStatusApproved
	record.FiatMintTxHash = txHash
	if err := s.escrowRepo.UpdateRedeem(ctx, record); err != nil {
		return nil, status.Errorf(codes.Internal, "update redeem: %v", err)
	}

	s.logger.Info("redeem approved", "redeem_id", req.RedeemId, "fiat_mint_tx_hash", txHash)
	return &pb.ApproveRedeemResponse{FiatMintTxHash: txHash}, nil
}

// RejectRedeem rejects a pending redeem request and returns Zeto tokens to the commercial bank.
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

	// Return Zeto tokens to the commercial bank's Paladin identity.
	if record.RequesterPaladinIdentity != "" {
		s.logger.Info("returning Zeto tokens on reject", "to", record.RequesterPaladinIdentity, "amount", record.Amount)
		if _, err := s.zeto.Transfer(ctx, record.RequesterPaladinIdentity, record.Amount); err != nil {
			s.logger.Error("failed to return Zeto tokens on reject — manual intervention may be needed",
				"redeem_id", req.RedeemId, "error", err)
		}
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
			Id:                       r.ID,
			RequesterId:              r.RequesterID,
			RequesterBesuAddress:     r.RequesterBesuAddress,
			RequesterPaladinIdentity: r.RequesterPaladinIdentity,
			Amount:                   r.Amount,
			Status:                   redeemStatusToProto(r.Status),
			ZetoTransferTxHash:       r.ZetoTransferTxHash,
			FiatMintTxHash:           r.FiatMintTxHash,
			RejectionReason:          r.RejectionReason,
			CreatedAt:                r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &pb.ListRedeemsResponse{Redeems: pbRecords}, nil
}

// --- Zeto Transfer (commercial bank proxy helper) ---

// InitiateZetoTransfer transfers Zeto tokens from the caller to a target identity.
// Used by the commercial bank's api-gateway before submitting a RequestRedeem to the central bank.
func (s *paymentOrchestratorService) InitiateZetoTransfer(ctx context.Context, req *pb.InitiateZetoTransferRequest) (*pb.InitiateZetoTransferResponse, error) {
	if req.ToIdentity == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "to_identity and amount are required")
	}

	s.logger.Info("initiating Zeto transfer", "to", req.ToIdentity, "amount", req.Amount)
	txHash, err := s.zeto.Transfer(ctx, req.ToIdentity, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto transfer: %v", err)
	}

	s.logger.Info("Zeto transfer completed", "to", req.ToIdentity, "tx_hash", txHash)
	return &pb.InitiateZetoTransferResponse{TxHash: txHash}, nil
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

func escrowStatusToProto(s domain.EscrowStatus) pb.EscrowStatus {
	switch s {
	case domain.EscrowStatusApproved:
		return pb.EscrowStatus_ESCROW_STATUS_APPROVED
	case domain.EscrowStatusRejected:
		return pb.EscrowStatus_ESCROW_STATUS_REJECTED
	default:
		return pb.EscrowStatus_ESCROW_STATUS_PENDING
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
