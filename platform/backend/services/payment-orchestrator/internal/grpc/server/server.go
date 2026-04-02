package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentOrchestratorService struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	zeto   ports.ZetoOperator
	htlc   ports.HTLCContractPort // on-chain HTLC coordination (may be nil)
	relay  ports.InteroperabilityPort
	logger *slog.Logger

	mu    sync.RWMutex
	htlcs map[string]*domain.HTLCRecord
}

// Config holds the dependencies for the gRPC server.
type Config struct {
	Zeto   ports.ZetoOperator
	HTLC   ports.HTLCContractPort // optional — nil disables on-chain coordination
	Relay  ports.InteroperabilityPort
	Logger *slog.Logger
}

// New builds a configured gRPC server with all payment-orchestrator handlers.
func New(cfg Config) *grpc.Server {
	svc := &paymentOrchestratorService{
		zeto:   cfg.Zeto,
		htlc:   cfg.HTLC,
		relay:  cfg.Relay,
		logger: cfg.Logger,
		htlcs:  make(map[string]*domain.HTLCRecord),
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPaymentOrchestratorServiceServer(grpcServer, svc)
	return grpcServer
}

// --- HTLC Dual-Layer Operations ---

func (s *paymentOrchestratorService) LockHTLC(ctx context.Context, req *pb.LockHTLCRequest) (*pb.LockHTLCResponse, error) {
	if req.AgreementId == "" || req.Receiver == "" || req.Amount == "" || req.TimeLock == 0 {
		return nil, status.Error(codes.InvalidArgument, "agreement_id, receiver, amount, and time_lock are required")
	}

	// 1. Generate secret and hashLock
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, status.Errorf(codes.Internal, "generate secret: %v", err)
	}
	secret := hex.EncodeToString(secretBytes)
	hashLockBytes := sha256.Sum256(secretBytes)
	hashLock := hex.EncodeToString(hashLockBytes[:])

	// 2. Lock tokens privately via Zeto
	s.logger.Info("locking Zeto tokens", "amount", req.Amount, "receiver", req.Receiver)
	lockResult, err := s.zeto.Lock(ctx, req.Amount, req.Receiver)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto lock: %v", err)
	}

	// 3. Generate contractId from agreement + timestamp
	contractIDBytes := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", req.AgreementId, time.Now().UnixNano())))
	contractID := hex.EncodeToString(contractIDBytes[:])

	// 4. Record on-chain HTLC coordination (if adapter configured)
	var htlcTxHash string
	if s.htlc != nil {
		var zetoRefBytes [32]byte
		if len(lockResult.LockedStateIDs) > 0 {
			zetoRefHash := sha256.Sum256([]byte(strings.Join(lockResult.LockedStateIDs, ",")))
			zetoRefBytes = zetoRefHash
		}
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLockBytes,
			TimeLock:    req.TimeLock,
			ZetoLockRef: zetoRefBytes,
		})
		if err != nil {
			s.logger.Warn("on-chain HTLC lock failed (Zeto lock succeeded)", "error", err)
		}
	}

	// 5. Store off-chain record
	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Receiver:    req.Receiver,
		Amount:      req.Amount,
		HashLock:    hashLock,
		TimeLock:    req.TimeLock,
		Secret:      secret,
		ZetoLockRef: strings.Join(lockResult.LockedStateIDs, ","),
		State:       domain.HTLCStateLocked,
		HTLCTxHash:  htlcTxHash,
		ZetoTxHash:  lockResult.TxHash,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.htlcs[contractID] = record
	s.mu.Unlock()

	s.logger.Info("HTLC locked",
		"contract_id", contractID,
		"hash_lock", hashLock,
		"zeto_lock_ref", lockResult.ZetoLockRef,
	)

	return &pb.LockHTLCResponse{
		ContractId: contractID,
		HashLock:   hashLock,
		HtlcTxHash: htlcTxHash,
		ZetoTxHash: lockResult.TxHash,
	}, nil
}

func (s *paymentOrchestratorService) LockHTLCWithHashLock(ctx context.Context, req *pb.LockHTLCWithHashLockRequest) (*pb.LockHTLCWithHashLockResponse, error) {
	if req.AgreementId == "" || req.Receiver == "" || req.Amount == "" || req.TimeLock == 0 || req.HashLock == "" {
		return nil, status.Error(codes.InvalidArgument, "agreement_id, receiver, amount, time_lock, and hash_lock are required")
	}

	// Decode the externally provided hashLock
	hashLockBytes, err := hex.DecodeString(req.HashLock)
	if err != nil || len(hashLockBytes) != 32 {
		return nil, status.Error(codes.InvalidArgument, "hash_lock must be a 64-char hex string (32 bytes)")
	}

	// Lock tokens privately via Zeto
	s.logger.Info("locking Zeto tokens (with external hashLock)", "amount", req.Amount, "receiver", req.Receiver)
	lockResult, err := s.zeto.Lock(ctx, req.Amount, req.Receiver)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto lock: %v", err)
	}

	// Generate contractId from agreement + timestamp
	contractIDBytes := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", req.AgreementId, time.Now().UnixNano())))
	contractID := hex.EncodeToString(contractIDBytes[:])

	// Record on-chain HTLC coordination (if adapter configured)
	var htlcTxHash string
	if s.htlc != nil {
		var hashLock32 [32]byte
		copy(hashLock32[:], hashLockBytes)
		var zetoRefBytes [32]byte
		if len(lockResult.LockedStateIDs) > 0 {
			zetoRefHash := sha256.Sum256([]byte(strings.Join(lockResult.LockedStateIDs, ",")))
			zetoRefBytes = zetoRefHash
		}
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLock32,
			TimeLock:    req.TimeLock,
			ZetoLockRef: zetoRefBytes,
		})
		if err != nil {
			s.logger.Warn("on-chain HTLC lock failed (Zeto lock succeeded)", "error", err)
		}
	}

	// Store off-chain record (no secret — only the initiator knows it)
	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Receiver:    req.Receiver,
		Amount:      req.Amount,
		HashLock:    req.HashLock,
		TimeLock:    req.TimeLock,
		ZetoLockRef: strings.Join(lockResult.LockedStateIDs, ","),
		State:       domain.HTLCStateLocked,
		HTLCTxHash:  htlcTxHash,
		ZetoTxHash:  lockResult.TxHash,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.htlcs[contractID] = record
	s.mu.Unlock()

	s.logger.Info("HTLC locked (external hashLock)",
		"contract_id", contractID,
		"hash_lock", req.HashLock,
		"zeto_lock_ref", lockResult.ZetoLockRef,
	)

	return &pb.LockHTLCWithHashLockResponse{
		ContractId: contractID,
		HashLock:   req.HashLock,
		HtlcTxHash: htlcTxHash,
		ZetoTxHash: lockResult.TxHash,
	}, nil
}

func (s *paymentOrchestratorService) SettleHTLC(ctx context.Context, req *pb.SettleHTLCRequest) (*pb.SettleHTLCResponse, error) {
	if req.ContractId == "" || req.Secret == "" {
		return nil, status.Error(codes.InvalidArgument, "contract_id and secret are required")
	}

	// Pre-compute hashLock from secret (needed for cross-spoke lookup).
	secretBytes, err := hex.DecodeString(req.Secret)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid secret hex")
	}
	hashBytes := sha256.Sum256(secretBytes)
	hashLock := hex.EncodeToString(hashBytes[:])

	s.mu.Lock()
	record, ok := s.htlcs[req.ContractId]
	if !ok {
		// Cross-spoke relay: the contract_id is from the other spoke.
		// Find the local HTLC that shares the same hashLock.
		for _, r := range s.htlcs {
			if r.HashLock == hashLock {
				if r.State == domain.HTLCStateLocked {
					record = r
					ok = true
					break
				}
				if r.State == domain.HTLCStateSettled {
					// Already settled (e.g. relay echo) — return idempotent success.
					s.mu.Unlock()
					s.logger.Info("SettleHTLC idempotent: already settled via hashLock match",
						"requested_contract_id", req.ContractId,
						"local_contract_id", r.ContractID,
					)
					return &pb.SettleHTLCResponse{
						HtlcTxHash: r.HTLCTxHash,
						ZetoTxHash: r.ZetoTxHash,
					}, nil
				}
			}
		}
	}
	if !ok {
		s.mu.Unlock()
		return nil, status.Errorf(codes.NotFound, "HTLC %q not found", req.ContractId)
	}

	if record.State == domain.HTLCStateSettled {
		// Direct contract_id match but already settled — return idempotent success.
		s.mu.Unlock()
		s.logger.Info("SettleHTLC idempotent: already settled", "contract_id", record.ContractID)
		return &pb.SettleHTLCResponse{
			HtlcTxHash: record.HTLCTxHash,
			ZetoTxHash: record.ZetoTxHash,
		}, nil
	}

	if record.State != domain.HTLCStateLocked {
		s.mu.Unlock()
		return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
	}

	// Verify secret matches hashLock
	if hashLock != record.HashLock {
		s.mu.Unlock()
		return nil, status.Error(codes.InvalidArgument, "secret does not match hashLock")
	}

	record.State = domain.HTLCStateSettled
	record.Secret = req.Secret
	record.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()

	// Settle on-chain HTLC (reveals secret via LogHTLCClaimed event)
	var htlcTxHash string
	if s.htlc != nil {
		var cid, sec [32]byte
		copy(cid[:], contractIDBytes(record.ContractID))
		copy(sec[:], secretBytes)
		htlcTxHash, err = s.htlc.Settle(ctx, cid, sec)
		if err != nil {
			s.logger.Warn("on-chain HTLC settle failed", "error", err)
		}
	}

	// Transfer locked Zeto tokens to receiver
	zetoTxHash, err := s.zeto.TransferLocked(ctx, record.ZetoLockRef, record.Receiver, record.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto transferLocked: %v", err)
	}

	s.logger.Info("HTLC settled", "contract_id", record.ContractID, "zeto_tx_hash", zetoTxHash, "htlc_tx_hash", htlcTxHash)

	// Update record with settle tx hashes so idempotent re-calls return them.
	s.mu.Lock()
	record.HTLCTxHash = htlcTxHash
	record.ZetoTxHash = zetoTxHash
	s.mu.Unlock()

	return &pb.SettleHTLCResponse{
		HtlcTxHash: htlcTxHash,
		ZetoTxHash: zetoTxHash,
	}, nil
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

	if record.State != domain.HTLCStateLocked {
		s.mu.Unlock()
		return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
	}

	if uint64(time.Now().Unix()) < record.TimeLock {
		s.mu.Unlock()
		return nil, status.Error(codes.FailedPrecondition, "time lock has not expired yet")
	}

	record.State = domain.HTLCStateRefunded
	record.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()

	// Refund on-chain HTLC coordination
	var htlcTxHash string
	if s.htlc != nil {
		var cid [32]byte
		copy(cid[:], contractIDBytes(req.ContractId))
		var htlcErr error
		htlcTxHash, htlcErr = s.htlc.Refund(ctx, cid)
		if htlcErr != nil {
			s.logger.Warn("on-chain HTLC refund failed", "error", htlcErr)
		}
	}

	zetoTxHash, err := s.zeto.Unlock(ctx, record.ZetoLockRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto unlock: %v", err)
	}

	s.logger.Info("HTLC refunded", "contract_id", req.ContractId, "zeto_tx_hash", zetoTxHash, "htlc_tx_hash", htlcTxHash)

	return &pb.RefundHTLCResponse{
		HtlcTxHash: htlcTxHash,
		ZetoTxHash: zetoTxHash,
	}, nil
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

	return &pb.GetHTLCStatusResponse{
		Lock: recordToProto(record),
	}, nil
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

// --- Token Operations (Zeto via Paladin) ---

func (s *paymentOrchestratorService) MintToken(ctx context.Context, req *pb.MintTokenRequest) (*pb.MintTokenResponse, error) {
	if req.To == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "to and amount are required")
	}

	txHash, err := s.zeto.Mint(ctx, req.To, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto mint: %v", err)
	}

	return &pb.MintTokenResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) TransferToken(ctx context.Context, req *pb.TransferTokenRequest) (*pb.TransferTokenResponse, error) {
	if req.To == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "to and amount are required")
	}

	txHash, err := s.zeto.Transfer(ctx, req.To, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto transfer: %v", err)
	}

	return &pb.TransferTokenResponse{TxHash: txHash}, nil
}

func (s *paymentOrchestratorService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	if req.Identity == "" {
		return nil, status.Error(codes.InvalidArgument, "identity is required")
	}

	balance, err := s.zeto.Balance(ctx, req.Identity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto balance: %v", err)
	}

	return &pb.GetBalanceResponse{Balance: balance}, nil
}

// --- FX Agreement Operations ---
// These will call the FXAgreement.sol Besu contract via the blockchain client.
// For now, they are stubs that return unimplemented.

func (s *paymentOrchestratorService) ProposeFXAgreement(_ context.Context, _ *pb.ProposeFXAgreementRequest) (*pb.ProposeFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "FX agreement operations require Besu contract integration — coming in next sprint")
}

func (s *paymentOrchestratorService) AcceptFXAgreement(_ context.Context, _ *pb.AcceptFXAgreementRequest) (*pb.AcceptFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *paymentOrchestratorService) RejectFXAgreement(_ context.Context, _ *pb.RejectFXAgreementRequest) (*pb.RejectFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *paymentOrchestratorService) CancelFXAgreement(_ context.Context, _ *pb.CancelFXAgreementRequest) (*pb.CancelFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *paymentOrchestratorService) SettleFXAgreement(_ context.Context, _ *pb.SettleFXAgreementRequest) (*pb.SettleFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *paymentOrchestratorService) GetFXAgreement(_ context.Context, _ *pb.GetFXAgreementRequest) (*pb.GetFXAgreementResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

func (s *paymentOrchestratorService) ListFXAgreements(_ context.Context, _ *pb.ListFXAgreementsRequest) (*pb.ListFXAgreementsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// --- Helpers ---

// contractIDBytes converts a hex-encoded contract ID string to a 32-byte array.
func contractIDBytes(hexStr string) []byte {
	b, _ := hex.DecodeString(hexStr)
	if len(b) > 32 {
		b = b[:32]
	}
	return b
}

func recordToProto(r *domain.HTLCRecord) *pb.HTLCLock {
	stateMap := map[domain.HTLCState]pb.HTLCState{
		domain.HTLCStateInvalid:  pb.HTLCState_HTLC_STATE_INVALID,
		domain.HTLCStatePending:  pb.HTLCState_HTLC_STATE_PENDING,
		domain.HTLCStateLocked:   pb.HTLCState_HTLC_STATE_LOCKED,
		domain.HTLCStateSettled:  pb.HTLCState_HTLC_STATE_SETTLED,
		domain.HTLCStateRefunded: pb.HTLCState_HTLC_STATE_REFUNDED,
	}
	return &pb.HTLCLock{
		ContractId:  r.ContractID,
		Sender:      r.Sender,
		Receiver:    r.Receiver,
		HashLock:    r.HashLock,
		TimeLock:    r.TimeLock,
		Secret:      r.Secret,
		ZetoLockRef: r.ZetoLockRef,
		State:       stateMap[r.State],
	}
}
