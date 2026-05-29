package server

import (
	"context"
	"crypto/rand"
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
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentOrchestratorService struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	token            ports.TCeBMPort               // on-chain tCeBM ERC-20 operations
	fiat             ports.FiatTokenPort            // on-chain fCeBM ERC-20 operations
	htlc             ports.HTLCContractPort         // on-chain HTLC coordination (may be nil)
	relay            ports.InteroperabilityPort
	escrowRepo       ports.EscrowRepository         // escrow flow persistence
	fxAgreementBesu  ports.FXAgreementContractPort  // FX agreement on Besu (optional)
	fxRepo           ports.FXAgreementRepository    // persistent FX agreement storage (nil = dev in-memory)
	rateTolPct       float64                        // rate tolerance fraction (e.g. 0.001 for 0.1%)
	strictHTLC       bool                           // when true, lock operations require verifiable agreement linkage
	logger           *slog.Logger

	mu           sync.RWMutex
	htlcs        map[string]*domain.HTLCRecord
	fxAgreements map[string]*domain.FXAgreementRecord
}

// Config holds the dependencies for the gRPC server.
type Config struct {
	Token            ports.TCeBMPort               // tCeBM ERC-20 adapter (required for escrow/redeem)
	Fiat             ports.FiatTokenPort            // fCeBM ERC-20 adapter (required for deposit approval)
	HTLC             ports.HTLCContractPort         // optional — nil disables on-chain coordination
	Relay            ports.InteroperabilityPort
	EscrowRepo       ports.EscrowRepository         // optional — nil disables escrow flow
	FXAgreementBesu  ports.FXAgreementContractPort  // optional — nil disables Besu FXAgreement path
	FXRepo           ports.FXAgreementRepository    // optional — nil falls back to in-memory map (dev)
	RateTolPct       float64                        // rate tolerance fraction, default 0.001 (0.1%)
	StrictHTLC       bool                           // strict Agreement-HTLC enforcement mode
	Logger           *slog.Logger
}

// New builds a configured gRPC server with all payment-orchestrator handlers.
func New(cfg Config) *grpc.Server {
	rateTol := cfg.RateTolPct
	if rateTol <= 0 {
		rateTol = 0.001 // default 0.1%
	}
	svc := &paymentOrchestratorService{
		token:           cfg.Token,
		fiat:            cfg.Fiat,
		htlc:            cfg.HTLC,
		relay:           cfg.Relay,
		escrowRepo:      cfg.EscrowRepo,
		fxAgreementBesu: cfg.FXAgreementBesu,
		fxRepo:          cfg.FXRepo,
		rateTolPct:      rateTol,
		strictHTLC:      cfg.StrictHTLC,
		logger:          cfg.Logger,
		htlcs:           make(map[string]*domain.HTLCRecord),
		fxAgreements:    make(map[string]*domain.FXAgreementRecord),
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPaymentOrchestratorServiceServer(grpcServer, svc)
	return grpcServer
}

// generateID returns a new UUID v4 string for record IDs.
func generateID() string {
	return uuid.New().String()
}

// --- HTLC Operations ---

func (s *paymentOrchestratorService) LockHTLC(ctx context.Context, req *pb.LockHTLCRequest) (*pb.LockHTLCResponse, error) {
	if req.Receiver == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "receiver and amount are required")
	}
	if req.AgreementId == "" {
		req.AgreementId = newUUID()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 3600
	}

	var agreementIDBytes [32]byte
	if s.strictHTLC && req.AgreementId == "" {
		return nil, status.Error(codes.FailedPrecondition, "agreement_id is required in strict mode")
	}
	if req.AgreementId != "" {
		fxRecord, err := s.getFXAgreement(ctx, req.AgreementId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
		}
		if fxRecord == nil {
			if s.strictHTLC {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement not found for provided agreement_id")
			}
		} else {
			if fxRecord.State != domain.FXStateAccepted {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement must be accepted before locking HTLC")
			}
			if fxRecord.ExpiryDate > 0 && uint64(time.Now().Unix()) > fxRecord.ExpiryDate {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement has expired")
			}
			if err := s.validateHTLCTermsAgainstAgreement(req.Receiver, req.Amount, fxRecord); err != nil {
				return nil, err
			}
			if s.fxAgreementBesu != nil {
				copy(agreementIDBytes[:], tradeIDBytes32(fxRecord.TradeID))
			} else if s.strictHTLC {
				agreementIDBytes = s.agreementCommitmentHash(fxRecord)
			}
		}
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, status.Errorf(codes.Internal, "generate secret: %v", err)
	}
	secret := hex.EncodeToString(secretBytes)
	hashLockBytes := sha256.Sum256(secretBytes)
	hashLock := hex.EncodeToString(hashLockBytes[:])

	contractIDBytes := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", req.AgreementId, time.Now().UnixNano())))
	contractID := hex.EncodeToString(contractIDBytes[:])

	var htlcTxHash string
	if s.htlc != nil {
		var emptyRef [32]byte
		var err error
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLockBytes,
			TimeLock:    req.TimeLock,
			ZetoLockRef: emptyRef,
			AgreementID: agreementIDBytes,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain HTLC lock failed: %v", err)
		}
	}

	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Receiver:    req.Receiver,
		Amount:      req.Amount,
		HashLock:    hashLock,
		TimeLock:    req.TimeLock,
		Secret:      secret,
		State:       domain.HTLCStateLocked,
		HTLCTxHash:  htlcTxHash,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.htlcs[contractID] = record
	s.mu.Unlock()

	s.logger.Info("HTLC locked", "contract_id", contractID, "hash_lock", hashLock)

	return &pb.LockHTLCResponse{
		ContractId: contractID,
		HashLock:   hashLock,
		HtlcTxHash: htlcTxHash,
	}, nil
}

func (s *paymentOrchestratorService) LockHTLCWithHashLock(ctx context.Context, req *pb.LockHTLCWithHashLockRequest) (*pb.LockHTLCWithHashLockResponse, error) {
	if req.Receiver == "" || req.Amount == "" || req.HashLock == "" {
		return nil, status.Error(codes.InvalidArgument, "receiver, amount, and hash_lock are required")
	}
	if req.AgreementId == "" {
		req.AgreementId = newUUID()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 1800
	}

	var agreementIDBytes [32]byte
	if req.AgreementId != "" {
		fxRecord, err := s.getFXAgreement(ctx, req.AgreementId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load FX agreement: %v", err)
		}
		if fxRecord != nil {
			if fxRecord.State != domain.FXStateAccepted {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement must be accepted before locking HTLC")
			}
			if fxRecord.ExpiryDate > 0 && uint64(time.Now().Unix()) > fxRecord.ExpiryDate {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement has expired")
			}
			if err := s.validateHTLCTermsAgainstAgreement(req.Receiver, req.Amount, fxRecord); err != nil {
				return nil, err
			}
			if s.fxAgreementBesu != nil {
				copy(agreementIDBytes[:], tradeIDBytes32(fxRecord.TradeID))
			} else if s.strictHTLC {
				agreementIDBytes = s.agreementCommitmentHash(fxRecord)
			}
		} else if s.strictHTLC {
			return nil, status.Error(codes.FailedPrecondition, "FX agreement not found for provided agreement_id")
		}
	}

	hashLockBytes, err := hex.DecodeString(req.HashLock)
	if err != nil || len(hashLockBytes) != 32 {
		return nil, status.Error(codes.InvalidArgument, "hash_lock must be a 64-char hex string (32 bytes)")
	}

	contractIDBytes := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", req.AgreementId, time.Now().UnixNano())))
	contractID := hex.EncodeToString(contractIDBytes[:])

	var htlcTxHash string
	if s.htlc != nil {
		var hashLock32 [32]byte
		copy(hashLock32[:], hashLockBytes)
		var emptyRef [32]byte
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLock32,
			TimeLock:    req.TimeLock,
			ZetoLockRef: emptyRef,
			AgreementID: agreementIDBytes,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain HTLC lock failed: %v", err)
		}
	}

	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Receiver:    req.Receiver,
		Amount:      req.Amount,
		HashLock:    req.HashLock,
		TimeLock:    req.TimeLock,
		State:       domain.HTLCStateLocked,
		HTLCTxHash:  htlcTxHash,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.htlcs[contractID] = record
	s.mu.Unlock()

	s.logger.Info("HTLC locked (external hashLock)", "contract_id", contractID, "hash_lock", req.HashLock)

	return &pb.LockHTLCWithHashLockResponse{
		ContractId: contractID,
		HashLock:   req.HashLock,
		HtlcTxHash: htlcTxHash,
	}, nil
}

func (s *paymentOrchestratorService) SettleHTLC(ctx context.Context, req *pb.SettleHTLCRequest) (*pb.SettleHTLCResponse, error) {
	if req.ContractId == "" || req.Secret == "" {
		return nil, status.Error(codes.InvalidArgument, "contract_id and secret are required")
	}

	secretBytes, err := hex.DecodeString(req.Secret)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid secret hex")
	}
	hashBytes := sha256.Sum256(secretBytes)
	hashLock := hex.EncodeToString(hashBytes[:])

	s.mu.Lock()
	record, ok := s.htlcs[req.ContractId]
	if !ok {
		for _, r := range s.htlcs {
			if r.HashLock == hashLock {
				if r.State == domain.HTLCStateLocked || r.State == domain.HTLCStateSettling {
					record = r
					ok = true
					break
				}
				if r.State == domain.HTLCStateSettled {
					s.mu.Unlock()
					s.logger.Info("SettleHTLC idempotent: already settled via hashLock match",
						"requested_contract_id", req.ContractId,
						"local_contract_id", r.ContractID,
					)
					return &pb.SettleHTLCResponse{HtlcTxHash: r.HTLCTxHash}, nil
				}
			}
		}
	}
	if !ok {
		s.mu.Unlock()
		return nil, status.Errorf(codes.NotFound, "HTLC %q not found", req.ContractId)
	}

	if record.State == domain.HTLCStateSettled {
		s.mu.Unlock()
		s.logger.Info("SettleHTLC idempotent: already settled", "contract_id", record.ContractID)
		return &pb.SettleHTLCResponse{HtlcTxHash: record.HTLCTxHash}, nil
	}

	if hashLock != record.HashLock {
		s.mu.Unlock()
		return nil, status.Error(codes.InvalidArgument, "secret does not match hashLock")
	}

	isRetry := record.State == domain.HTLCStateSettling

	if !isRetry {
		if record.State != domain.HTLCStateLocked {
			s.mu.Unlock()
			return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
		}
		record.State = domain.HTLCStateSettling
		record.Secret = req.Secret
		record.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()

	var htlcTxHash string
	if s.htlc != nil && !isRetry {
		var cid, sec [32]byte
		copy(cid[:], contractIDBytes(record.ContractID))
		copy(sec[:], secretBytes)
		htlcTxHash, err = s.htlc.Settle(ctx, cid, sec)
		if err != nil {
			s.logger.Error("on-chain HTLC settle failed — rolling back to LOCKED", "contract_id", record.ContractID, "error", err)
			s.mu.Lock()
			record.State = domain.HTLCStateLocked
			record.UpdatedAt = time.Now().UTC()
			s.mu.Unlock()
			return nil, status.Errorf(codes.Internal, "on-chain HTLC settle failed: %v", err)
		}
		s.mu.Lock()
		record.HTLCTxHash = htlcTxHash
		s.mu.Unlock()
	} else if isRetry {
		htlcTxHash = record.HTLCTxHash
	}

	s.mu.Lock()
	record.State = domain.HTLCStateSettled
	record.UpdatedAt = time.Now().UTC()
	record.HTLCTxHash = htlcTxHash
	s.mu.Unlock()

	s.logger.Info("HTLC settled", "contract_id", record.ContractID, "htlc_tx_hash", htlcTxHash)

	return &pb.SettleHTLCResponse{HtlcTxHash: htlcTxHash}, nil
}

func (s *paymentOrchestratorService) RefundHTLC(ctx context.Context, req *pb.RefundHTLCRequest) (*pb.RefundHTLCResponse, error) {
	if req.ContractId == "" {
		return nil, status.Error(codes.InvalidArgument, "contract_id is required")
	}

	s.mu.Lock()
	record, ok := s.htlcs[req.ContractId]
	if !ok {
		s.mu.Unlock()
		return nil, status.Errorf(codes.NotFound, "HTLC %q not found", req.ContractId)
	}

	if record.State == domain.HTLCStateRefunded {
		s.mu.Unlock()
		return &pb.RefundHTLCResponse{HtlcTxHash: record.HTLCTxHash}, nil
	}

	isRetry := record.State == domain.HTLCStateRefunding

	if !isRetry {
		if record.State != domain.HTLCStateLocked {
			s.mu.Unlock()
			return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
		}

		if uint64(time.Now().Unix()) < record.TimeLock {
			s.mu.Unlock()
			return nil, status.Error(codes.FailedPrecondition, "time lock has not expired yet")
		}

		record.State = domain.HTLCStateRefunding
		record.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()

	var htlcTxHash string
	if s.htlc != nil && !isRetry {
		var cid [32]byte
		copy(cid[:], contractIDBytes(req.ContractId))
		var htlcErr error
		htlcTxHash, htlcErr = s.htlc.Refund(ctx, cid)
		if htlcErr != nil {
			s.logger.Warn("on-chain HTLC refund failed", "error", htlcErr)
		}
		s.mu.Lock()
		record.HTLCTxHash = htlcTxHash
		s.mu.Unlock()
	} else if isRetry {
		htlcTxHash = record.HTLCTxHash
	}

	s.mu.Lock()
	record.State = domain.HTLCStateRefunded
	record.UpdatedAt = time.Now().UTC()
	record.HTLCTxHash = htlcTxHash
	s.mu.Unlock()

	s.logger.Info("HTLC refunded", "contract_id", req.ContractId, "htlc_tx_hash", htlcTxHash)

	return &pb.RefundHTLCResponse{HtlcTxHash: htlcTxHash}, nil
}

func (s *paymentOrchestratorService) GetHTLCStatus(_ context.Context, req *pb.GetHTLCStatusRequest) (*pb.GetHTLCStatusResponse, error) {
	if req.ContractId == "" {
		return nil, status.Error(codes.InvalidArgument, "contract_id is required")
	}

	s.mu.RLock()
	record, ok := s.htlcs[req.ContractId]
	s.mu.RUnlock()

	if !ok {
		return nil, status.Errorf(codes.NotFound, "HTLC %q not found", req.ContractId)
	}

	return &pb.GetHTLCStatusResponse{Lock: recordToProto(record)}, nil
}

func (s *paymentOrchestratorService) SearchHTLC(_ context.Context, req *pb.SearchHTLCRequest) (*pb.SearchHTLCResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*pb.HTLCLock
	for _, r := range s.htlcs {
		if req.AgreementId != "" && r.AgreementID != req.AgreementId {
			continue
		}
		if req.Sender != "" && r.Sender != req.Sender {
			continue
		}
		if req.Receiver != "" && r.Receiver != req.Receiver {
			continue
		}
		if req.State != "" && string(r.State) != req.State {
			continue
		}
		results = append(results, recordToProto(r))
	}

	return &pb.SearchHTLCResponse{Locks: results}, nil
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

	return &pb.GetBalanceResponse{Balance: balance}, nil
}

// --- FX Agreement Operations ---

func (s *paymentOrchestratorService) ProposeFXAgreement(ctx context.Context, req *pb.ProposeFXAgreementRequest) (*pb.ProposeFXAgreementResponse, error) {
	if req.CounterpartyB == "" || req.OriginAmount == "" || req.CounterAmount == "" ||
		req.OriginCurrency == "" || req.CounterCurrency == "" || req.Rate == "" || req.ExpiryDate == 0 {
		return nil, status.Error(codes.InvalidArgument, "counterparty_b, origin_amount, counter_amount, origin_currency, counter_currency, rate, and expiry_date are required")
	}
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
	if s.htlc != nil && s.strictHTLC && s.fxAgreementBesu == nil {
		if _, err := s.htlc.RegisterAgreementCommitment(ctx, s.agreementCommitmentHash(record)); err != nil {
			return nil, status.Errorf(codes.Internal, "register agreement commitment: %v", err)
		}
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
		f := ports.FXAgreementFilter{Counterparty: req.Counterparty}
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

func contractIDBytes(hexStr string) []byte {
	b, _ := hex.DecodeString(hexStr)
	if len(b) > 32 {
		b = b[:32]
	}
	return b
}

func recordToProto(r *domain.HTLCRecord) *pb.HTLCLock {
	stateMap := map[domain.HTLCState]pb.HTLCState{
		domain.HTLCStateInvalid:   pb.HTLCState_HTLC_STATE_INVALID,
		domain.HTLCStatePending:   pb.HTLCState_HTLC_STATE_PENDING,
		domain.HTLCStateLocked:    pb.HTLCState_HTLC_STATE_LOCKED,
		domain.HTLCStateSettled:   pb.HTLCState_HTLC_STATE_SETTLED,
		domain.HTLCStateRefunded:  pb.HTLCState_HTLC_STATE_REFUNDED,
		domain.HTLCStateSettling:  pb.HTLCState_HTLC_STATE_SETTLING,
		domain.HTLCStateRefunding: pb.HTLCState_HTLC_STATE_REFUNDING,
	}
	return &pb.HTLCLock{
		ContractId: r.ContractID,
		Sender:     r.Sender,
		Receiver:   r.Receiver,
		HashLock:   r.HashLock,
		TimeLock:   r.TimeLock,
		Secret:     r.Secret,
		State:      stateMap[r.State],
	}
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

func (s *paymentOrchestratorService) agreementCommitmentHash(rec *domain.FXAgreementRecord) [32]byte {
	payload := []byte(rec.TradeID + "|" + rec.OriginAmount + "|" + rec.CounterAmount + "|" + rec.Rate)
	h := gethcrypto.Keccak256Hash(payload)
	var out [32]byte
	copy(out[:], h.Bytes())
	return out
}

func (s *paymentOrchestratorService) validateHTLCTermsAgainstAgreement(
	receiver, amount string,
	fx *domain.FXAgreementRecord,
) error {
	var expectedReceiver, expectedAmount string
	if fx.SpokeAReceiver != "" || fx.SpokeBReceiver != "" {
		// Use SpokeA/B receivers based on which is set for this spoke's role
		if fx.SpokeAReceiver != "" && fx.SpokeBReceiver == "" {
			expectedReceiver = fx.SpokeAReceiver
			expectedAmount = fx.OriginAmount
		} else if fx.SpokeBReceiver != "" && fx.SpokeAReceiver == "" {
			expectedReceiver = fx.SpokeBReceiver
			expectedAmount = fx.CounterAmount
		}
	}

	if expectedReceiver != "" && receiver != expectedReceiver {
		return status.Errorf(codes.FailedPrecondition,
			"HTLC receiver %q does not match FX agreement receiver %q",
			receiver, expectedReceiver)
	}
	if expectedAmount != "" && amount != expectedAmount {
		return status.Errorf(codes.FailedPrecondition,
			"HTLC amount %q does not match FX agreement amount %q",
			amount, expectedAmount)
	}
	return nil
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
