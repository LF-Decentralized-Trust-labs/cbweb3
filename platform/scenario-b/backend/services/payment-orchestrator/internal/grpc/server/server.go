// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentOrchestratorService struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	token           ports.TCeBMPort     // on-chain tCeBM ERC-20 operations
	fiat            ports.FiatTokenPort // on-chain fCeBM ERC-20 operations
	relay           ports.InteroperabilityPort
	escrowRepo      ports.EscrowRepository        // escrow flow persistence
	fxAgreementBesu ports.FXAgreementContractPort // FX agreement on Besu (optional)
	fxRepo          ports.FXAgreementRepository   // persistent FX agreement storage (nil = dev in-memory)
	rateTolPct      float64                       // rate tolerance fraction (e.g. 0.001 for 0.1%)
	logger          *slog.Logger

	mu           sync.RWMutex
	fxAgreements map[string]*domain.FXAgreementRecord
}

// Config holds the dependencies for the gRPC server.
type Config struct {
	Token           ports.TCeBMPort     // tCeBM ERC-20 adapter (required for escrow/redeem)
	Fiat            ports.FiatTokenPort // fCeBM ERC-20 adapter (required for deposit approval)
	Relay           ports.InteroperabilityPort
	EscrowRepo      ports.EscrowRepository        // optional — nil disables escrow flow
	FXAgreementBesu ports.FXAgreementContractPort // optional — nil disables Besu FXAgreement path
	FXRepo          ports.FXAgreementRepository   // optional — nil falls back to in-memory map (dev)
	RateTolPct      float64                       // rate tolerance fraction, default 0.001 (0.1%)
	Logger          *slog.Logger
}

// New builds a configured gRPC server with all payment-orchestrator handlers.
// R2-H-8: the returned server installs authorization interceptors (and mutual TLS
// when the GRPC_MTLS_* env vars are set). With nothing set it runs in audit mode
// with NO caller authentication so existing plaintext callers keep working; the
// x-caller-identity header is trusted only under GRPC_AUTHZ_ALLOW_HEADER_IDENTITY
// (transitional). GRPC_AUTHZ_ENFORCE (which requires mTLS) rejects unauthenticated
// callers. An error is returned on a fail-open misconfiguration (partial mTLS
// material, or enforcement requested without mTLS).
func New(cfg Config) (*grpc.Server, error) {
	rateTol := cfg.RateTolPct
	if rateTol <= 0 {
		rateTol = 0.001 // default 0.1%
	}
	svc := &paymentOrchestratorService{
		token:           cfg.Token,
		fiat:            cfg.Fiat,
		relay:           cfg.Relay,
		escrowRepo:      cfg.EscrowRepo,
		fxAgreementBesu: cfg.FXAgreementBesu,
		fxRepo:          cfg.FXRepo,
		rateTolPct:      rateTol,
		logger:          cfg.Logger,
		fxAgreements:    make(map[string]*domain.FXAgreementRecord),
	}
	serverOpts, err := authz.ServerOptionsFromEnv(cfg.Logger, nil)
	if err != nil {
		return nil, fmt.Errorf("configure gRPC security: %w", err)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterPaymentOrchestratorServiceServer(grpcServer, svc)
	return grpcServer, nil
}

func generateID() string {
	return uuid.New().String()
}

// --- Token Balance ---

func (s *paymentOrchestratorService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	if s.token == nil {
		return nil, status.Error(codes.Unavailable, "tCeBM token adapter not configured")
	}

	var (
		balance string
		err     error
	)
	if req.Address != "" {
		balance, err = s.token.BalanceOf(ctx, req.Address)
	} else {
		balance, err = s.token.GetBalance(ctx)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "tCeBM balance: %v", err)
	}

	decimals, err := s.token.Decimals(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "tCeBM decimals: %v", err)
	}

	// Symbol is the single source of truth for the currency code shown in the UI,
	// but it is non-essential to the balance value itself: a read failure must not
	// fail the whole call. Clients fall back to their configured fiat symbol.
	symbol, err := s.token.Symbol(ctx)
	if err != nil {
		s.logger.Warn("tCeBM symbol read failed; returning balance without symbol", "error", err)
		symbol = ""
	}

	return &pb.GetBalanceResponse{Balance: balance, Decimals: uint32(decimals), Symbol: symbol}, nil
}

// --- FX Agreement Operations ---

func (s *paymentOrchestratorService) ProposeFXAgreement(ctx context.Context, req *pb.ProposeFXAgreementRequest) (*pb.ProposeFXAgreementResponse, error) {
	if req.CounterpartyB == "" || req.OriginAmount == "" || req.CounterAmount == "" ||
		req.OriginCurrency == "" || req.CounterCurrency == "" || req.Rate == "" || req.ExpiryDate == 0 {
		return nil, status.Error(codes.InvalidArgument, "counterparty_b, origin_amount, counter_amount, origin_currency, counter_currency, rate, and expiry_date are required")
	}
	// #nosec G115 -- unix timestamp is always positive and fits uint64
	if req.ExpiryDate <= uint64(time.Now().Unix()) {
		return nil, status.Error(codes.InvalidArgument, "expiry_date must be in the future")
	}
	if strings.EqualFold(req.OriginCurrency, req.CounterCurrency) {
		return nil, status.Error(codes.InvalidArgument, "origin_currency and counter_currency must be different")
	}
	originRat, ok := new(big.Rat).SetString(req.OriginAmount)
	if !ok || originRat.Sign() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "origin_amount must be a positive decimal")
	}
	counterRat, ok := new(big.Rat).SetString(req.CounterAmount)
	if !ok || counterRat.Sign() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "counter_amount must be a positive decimal")
	}
	rateRat, ok := new(big.Rat).SetString(req.Rate)
	if !ok || rateRat.Sign() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "rate must be a positive decimal")
	}
	expected := new(big.Rat).Quo(counterRat, originRat)
	diff := new(big.Rat).Sub(expected, rateRat)
	if diff.Sign() < 0 {
		diff.Neg(diff)
	}
	tol := new(big.Rat).SetFloat64(s.rateTolPct)
	if diff.Cmp(tol) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "rate inconsistent with amounts: abs(counter/origin-rate) exceeds tolerance %.6f", s.rateTolPct)
	}

	tradeID := req.TradeId
	if tradeID == "" {
		tradeID = newUUID()
	}

	var txHash string

	now := time.Now().UTC()
	record := &domain.FXAgreementRecord{
		TradeID:         tradeID,
		Originator:      req.Originator,
		CounterpartyB:   req.CounterpartyB,
		SettlementAgent: req.SettlementAgent,
		Custodian:       req.Custodian,
		Beneficiary:     req.Beneficiary,
		OriginAmount:    req.OriginAmount,
		CounterAmount:   req.CounterAmount,
		OriginCurrency:  req.OriginCurrency,
		CounterCurrency: req.CounterCurrency,
		Rate:            req.Rate,
		SpokeAReceiver:  req.SpokeAReceiver,
		SpokeBReceiver:  req.SpokeBReceiver,
		ExpiryDate:      req.ExpiryDate,
		State:           domain.FXStateProposed,
		OnChainTxHash:   txHash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.saveFXAgreement(ctx, record, true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist FX agreement: %v", err)
	}
	s.appendFXAuditEvent(ctx, tradeID, domain.FXStateInvalid, domain.FXStateProposed, txHash, req.OnBehalf)

	s.logger.Info("FX agreement proposed", "trade_id", tradeID, "on_behalf", req.OnBehalf)

	return &pb.ProposeFXAgreementResponse{
		TradeId: tradeID,
		TxHash:  txHash,
	}, nil
}

func (s *paymentOrchestratorService) AcceptFXAgreement(ctx context.Context, req *pb.AcceptFXAgreementRequest) (*pb.AcceptFXAgreementResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	record, err := s.getFXAgreement(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "FX agreement %q not found", req.TradeId)
	}
	if record.State != domain.FXStateProposed {
		return nil, status.Errorf(codes.FailedPrecondition, "FX agreement %q is in state %s, expected PROPOSED", req.TradeId, record.State)
	}

	var txHash string
	if s.fxAgreementBesu != nil {
		fxCtx := ctx
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		if req.OnBehalf {
			txHash, err = s.fxAgreementBesu.AcceptOnBehalf(fxCtx, tradeIDBytes)
		} else {
			txHash, err = s.fxAgreementBesu.Accept(fxCtx, tradeIDBytes)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain FX accept: %v", err)
		}
	}

	from := record.State
	record.State = domain.FXStateAccepted
	record.OnChainTxHash = txHash
	record.UpdatedAt = time.Now().UTC()
	if err := s.saveFXAgreement(ctx, record, false); err != nil {
		return nil, status.Errorf(codes.Internal, "persist FX agreement: %v", err)
	}
	s.appendFXAuditEvent(ctx, req.TradeId, from, domain.FXStateAccepted, txHash, req.OnBehalf)

	s.logger.Info("FX agreement accepted", "trade_id", req.TradeId, "on_behalf", req.OnBehalf)

	return &pb.AcceptFXAgreementResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) RejectFXAgreement(ctx context.Context, req *pb.RejectFXAgreementRequest) (*pb.RejectFXAgreementResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	record, err := s.getFXAgreement(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "FX agreement %q not found", req.TradeId)
	}
	if record.State != domain.FXStateProposed {
		return nil, status.Errorf(codes.FailedPrecondition, "FX agreement %q is in state %s, expected PROPOSED", req.TradeId, record.State)
	}

	var txHash string
	if s.fxAgreementBesu != nil {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		if req.OnBehalf {
			txHash, err = s.fxAgreementBesu.RejectOnBehalf(ctx, tradeIDBytes)
		} else {
			txHash, err = s.fxAgreementBesu.Reject(ctx, tradeIDBytes)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain FX reject: %v", err)
		}
	}

	from := record.State
	record.State = domain.FXStateRejected
	record.OnChainTxHash = txHash
	record.UpdatedAt = time.Now().UTC()
	if err := s.saveFXAgreement(ctx, record, false); err != nil {
		return nil, status.Errorf(codes.Internal, "persist FX agreement: %v", err)
	}
	s.appendFXAuditEvent(ctx, req.TradeId, from, domain.FXStateRejected, txHash, req.OnBehalf)

	s.logger.Info("FX agreement rejected", "trade_id", req.TradeId, "on_behalf", req.OnBehalf)

	return &pb.RejectFXAgreementResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) CancelFXAgreement(ctx context.Context, req *pb.CancelFXAgreementRequest) (*pb.CancelFXAgreementResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	record, err := s.getFXAgreement(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "FX agreement %q not found", req.TradeId)
	}
	if record.State != domain.FXStateProposed && record.State != domain.FXStateAccepted {
		return nil, status.Errorf(codes.FailedPrecondition, "FX agreement %q is in state %s, expected PROPOSED or ACCEPTED", req.TradeId, record.State)
	}

	var txHash string
	if s.fxAgreementBesu != nil {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		txHash, err = s.fxAgreementBesu.Cancel(ctx, tradeIDBytes)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain FX cancel: %v", err)
		}
	}

	from := record.State
	record.State = domain.FXStateCancelled
	record.OnChainTxHash = txHash
	record.UpdatedAt = time.Now().UTC()
	if err := s.saveFXAgreement(ctx, record, false); err != nil {
		return nil, status.Errorf(codes.Internal, "persist FX agreement: %v", err)
	}
	s.appendFXAuditEvent(ctx, req.TradeId, from, domain.FXStateCancelled, txHash, false)

	s.logger.Info("FX agreement cancelled", "trade_id", req.TradeId)

	return &pb.CancelFXAgreementResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) SettleFXAgreement(ctx context.Context, req *pb.SettleFXAgreementRequest) (*pb.SettleFXAgreementResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	record, err := s.getFXAgreement(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "FX agreement %q not found", req.TradeId)
	}
	if record.State != domain.FXStateAccepted {
		return nil, status.Errorf(codes.FailedPrecondition, "FX agreement %q is in state %s, expected ACCEPTED", req.TradeId, record.State)
	}

	var txHash string
	if s.fxAgreementBesu != nil {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		txHash, err = s.fxAgreementBesu.Settle(ctx, tradeIDBytes)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain FX settle: %v", err)
		}
	}

	from := record.State
	record.State = domain.FXStateSettled
	record.OnChainTxHash = txHash
	record.UpdatedAt = time.Now().UTC()
	if err := s.saveFXAgreement(ctx, record, false); err != nil {
		return nil, status.Errorf(codes.Internal, "persist FX agreement: %v", err)
	}
	s.appendFXAuditEvent(ctx, req.TradeId, from, domain.FXStateSettled, txHash, false)

	s.logger.Info("FX agreement settled", "trade_id", req.TradeId)

	return &pb.SettleFXAgreementResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) GetFXAgreement(ctx context.Context, req *pb.GetFXAgreementRequest) (*pb.GetFXAgreementResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	record, err := s.getFXAgreement(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "FX agreement %q not found", req.TradeId)
	}

	return &pb.GetFXAgreementResponse{Agreement: fxRecordToProto(record)}, nil
}

func (s *paymentOrchestratorService) ListFXAgreements(ctx context.Context, req *pb.ListFXAgreementsRequest) (*pb.ListFXAgreementsResponse, error) {
	var results []*pb.FXAgreement
	stateFilter := strings.TrimPrefix(req.State, "FX_STATE_")

	if s.fxRepo != nil {
		// Explicit, not left to the repository's clamp: the response is capped at one page
		// and ListFXAgreementsRequest carries no page size, so a node with more agreements
		// than this returns only the newest MaxFXAgreementPageSize of them. Naming the bound
		// here is what keeps that visible to anyone reading this handler (finding R2-M-14).
		f := ports.FXAgreementFilter{Counterparty: req.Counterparty, Limit: ports.MaxFXAgreementPageSize}
		if stateFilter != "" {
			f.State = domain.FXState(stateFilter)
		}
		recs, err := s.fxRepo.ListAgreements(ctx, f)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list FX agreements: %v", err)
		}
		for _, r := range recs {
			results = append(results, fxRecordToProto(r))
		}
		return &pb.ListFXAgreementsResponse{Agreements: results}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.fxAgreements {
		if req.Counterparty != "" && r.CounterpartyB != req.Counterparty && r.Originator != req.Counterparty {
			continue
		}
		if stateFilter != "" && string(r.State) != stateFilter {
			continue
		}
		results = append(results, fxRecordToProto(r))
	}
	return &pb.ListFXAgreementsResponse{Agreements: results}, nil
}

func (s *paymentOrchestratorService) ListFXAgreementEvents(ctx context.Context, req *pb.ListFXAgreementEventsRequest) (*pb.ListFXAgreementEventsResponse, error) {
	if req.TradeId == "" {
		return nil, status.Error(codes.InvalidArgument, "trade_id is required")
	}

	if s.fxRepo == nil {
		return nil, status.Error(codes.FailedPrecondition, "FX agreement repository not configured")
	}

	events, err := s.fxRepo.ListAuditEvents(ctx, req.TradeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list FX audit events: %v", err)
	}

	pbEvents := make([]*pb.FXAgreementEvent, 0, len(events))
	for _, e := range events {
		pbEvents = append(pbEvents, fxEventToProto(e))
	}

	return &pb.ListFXAgreementEventsResponse{Events: pbEvents}, nil
}

// --- Helpers ---

func newUUID() string {
	return uuid.NewString()
}

func fxRecordToProto(r *domain.FXAgreementRecord) *pb.FXAgreement {
	stateMap := map[domain.FXState]pb.FXAgreementState{
		domain.FXStateInvalid:   pb.FXAgreementState_FX_STATE_INVALID,
		domain.FXStateProposed:  pb.FXAgreementState_FX_STATE_PROPOSED,
		domain.FXStateAccepted:  pb.FXAgreementState_FX_STATE_ACCEPTED,
		domain.FXStateRejected:  pb.FXAgreementState_FX_STATE_REJECTED,
		domain.FXStateCancelled: pb.FXAgreementState_FX_STATE_CANCELLED,
		domain.FXStateSettled:   pb.FXAgreementState_FX_STATE_SETTLED,
	}
	return &pb.FXAgreement{
		TradeId:         r.TradeID,
		Originator:      r.Originator,
		CounterpartyB:   r.CounterpartyB,
		SettlementAgent: r.SettlementAgent,
		Custodian:       r.Custodian,
		Beneficiary:     r.Beneficiary,
		OriginAmount:    r.OriginAmount,
		CounterAmount:   r.CounterAmount,
		OriginCurrency:  r.OriginCurrency,
		CounterCurrency: r.CounterCurrency,
		SpokeAReceiver:  r.SpokeAReceiver,
		SpokeBReceiver:  r.SpokeBReceiver,
		Rate:            r.Rate,
		ExpiryDate:      r.ExpiryDate,
		State:           stateMap[r.State],
		GroupId:         r.GroupID,
		ContractAddress: r.ContractAddress,
	}
}

func fxEventToProto(e *domain.FXAgreementEvent) *pb.FXAgreementEvent {
	stateMap := map[domain.FXState]pb.FXAgreementState{
		domain.FXStateInvalid:   pb.FXAgreementState_FX_STATE_INVALID,
		domain.FXStateProposed:  pb.FXAgreementState_FX_STATE_PROPOSED,
		domain.FXStateAccepted:  pb.FXAgreementState_FX_STATE_ACCEPTED,
		domain.FXStateRejected:  pb.FXAgreementState_FX_STATE_REJECTED,
		domain.FXStateCancelled: pb.FXAgreementState_FX_STATE_CANCELLED,
		domain.FXStateSettled:   pb.FXAgreementState_FX_STATE_SETTLED,
	}

	sourceMap := map[domain.EventSource]pb.FXAgreementEventSource{
		domain.EventSourceLocalAPI:  pb.FXAgreementEventSource_FX_EVENT_SOURCE_LOCAL_API,
		domain.EventSourceRelay:     pb.FXAgreementEventSource_FX_EVENT_SOURCE_RELAY,
		domain.EventSourceSystemJob: pb.FXAgreementEventSource_FX_EVENT_SOURCE_SYSTEM_JOB,
		domain.EventSourceOnBehalf:  pb.FXAgreementEventSource_FX_EVENT_SOURCE_ON_BEHALF,
	}

	return &pb.FXAgreementEvent{
		Id:             e.ID,
		TradeId:        e.TradeID,
		FromState:      stateMap[e.FromState],
		ToState:        stateMap[e.ToState],
		Actor:          e.Actor,
		OccurredAtUnix: e.OccurredAt.Unix(),
		Notes:          e.Notes,
		TxHash:         e.TxHash,
		Source:         sourceMap[e.Source],
	}
}

func (s *paymentOrchestratorService) getFXAgreement(ctx context.Context, tradeID string) (*domain.FXAgreementRecord, error) {
	if s.fxRepo != nil {
		rec, err := s.fxRepo.GetAgreement(ctx, tradeID)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			return rec, nil
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if rec, ok := s.fxAgreements[tradeID]; ok {
		return rec, nil
	}
	return nil, nil
}

func (s *paymentOrchestratorService) saveFXAgreement(ctx context.Context, rec *domain.FXAgreementRecord, isCreate bool) error {
	if s.fxRepo != nil {
		if isCreate {
			if err := s.fxRepo.CreateAgreement(ctx, rec); err != nil {
				return err
			}
		} else {
			if err := s.fxRepo.UpdateAgreement(ctx, rec); err != nil {
				return err
			}
		}
	}
	s.mu.Lock()
	s.fxAgreements[rec.TradeID] = rec
	s.mu.Unlock()
	return nil
}

func (s *paymentOrchestratorService) appendFXAuditEvent(ctx context.Context, tradeID string, from, to domain.FXState, txHash string, onBehalf bool) {
	if s.fxRepo == nil {
		return
	}
	source := domain.EventSourceLocalAPI
	if onBehalf {
		source = domain.EventSourceOnBehalf
	}
	_ = s.fxRepo.CreateAuditEvent(ctx, &domain.FXAgreementEvent{
		TradeID:    tradeID,
		FromState:  from,
		ToState:    to,
		Actor:      "payment-orchestrator",
		OccurredAt: time.Now().UTC(),
		TxHash:     txHash,
		Source:     source,
	})
}

func tradeIDBytes32(tradeID string) []byte {
	b, err := hex.DecodeString(tradeID)
	if err == nil && len(b) == 32 {
		return b
	}
	h := sha256.Sum256([]byte(tradeID))
	return h[:]
}

// buildFXProposalParams converts gRPC request fields to the on-chain proposal params.
func buildFXProposalParams(tradeID string, req *pb.ProposeFXAgreementRequest) (ports.FXProposalParams, error) {
	var tid [32]byte
	copy(tid[:], tradeIDBytes32(tradeID))

	originAmount, ok := new(big.Int).SetString(req.OriginAmount, 10)
	if !ok {
		return ports.FXProposalParams{}, fmt.Errorf("invalid origin_amount: %s", req.OriginAmount)
	}
	counterAmount, ok := new(big.Int).SetString(req.CounterAmount, 10)
	if !ok {
		return ports.FXProposalParams{}, fmt.Errorf("invalid counter_amount: %s", req.CounterAmount)
	}
	rate, ok := new(big.Int).SetString(req.Rate, 10)
	if !ok {
		return ports.FXProposalParams{}, fmt.Errorf("invalid rate: %s", req.Rate)
	}

	var originCurrency, counterCurrency [32]byte
	copy(originCurrency[:], []byte(req.OriginCurrency))
	copy(counterCurrency[:], []byte(req.CounterCurrency))

	return ports.FXProposalParams{
		TradeID:         tid,
		Originator:      common.HexToAddress(req.Originator),
		CounterpartyB:   common.HexToAddress(req.CounterpartyB),
		SettlementAgent: common.HexToAddress(req.SettlementAgent),
		Custodian:       common.HexToAddress(req.Custodian),
		Beneficiary:     common.HexToAddress(req.Beneficiary),
		OriginAmount:    originAmount,
		CounterAmount:   counterAmount,
		OriginCurrency:  originCurrency,
		CounterCurrency: counterCurrency,
		Rate:            rate,
		ExpiryDate:      new(big.Int).SetUint64(req.ExpiryDate),
	}, nil
}
