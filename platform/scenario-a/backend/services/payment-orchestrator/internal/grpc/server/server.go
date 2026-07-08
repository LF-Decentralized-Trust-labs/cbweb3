// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type paymentOrchestratorService struct {
	pb.UnimplementedPaymentOrchestratorServiceServer
	zeto             ports.ZetoOperator
	htlc             ports.HTLCContractPort // on-chain HTLC coordination (may be nil)
	relay            ports.InteroperabilityPort
	fiat             ports.FiatTokenPort           // on-chain fCeBM operations (may be nil)
	escrowRepo       ports.EscrowRepository        // escrow flow persistence
	fxAgreementBesu  ports.FXAgreementContractPort // FX agreement on Besu (optional)
	fxAgreementPente ports.FXAgreementContractPort // FX agreement on Pente private context (optional)
	fxRepo           ports.FXAgreementRepository   // persistent FX agreement storage (nil = dev in-memory)
	htlcRepo         ports.HTLCRepository          // required in production; nil only in unit tests
	pente            ports.PenteClientPort         // optional bilateral private-context manager
	fxContexts       *fxContextStore               // A6: file-backed FX context resolver (group/contract per bank)
	rateTolPct       float64                       // rate tolerance fraction (e.g. 0.001 for 0.1%)
	crossSpokeMode   bool                          // when true, settle is gated on CounterpartyLocked
	strictHTLC       bool                          // when true, lock operations require verifiable agreement linkage
	spokePrefix      string                        // e.g. "spoke-a" — extracted from PALADIN_IDENTITY
	paladinIdentity  string                        // full identity, e.g. "funded_operator@spoke-a-bank-a"
	logger           *slog.Logger

	mu           sync.RWMutex
	htlcs        map[string]*domain.HTLCRecord
	fxAgreements map[string]*domain.FXAgreementRecord // temporary fallback cache while repository migration is incremental
}

// Config holds the dependencies for the gRPC server.
type Config struct {
	Zeto             ports.ZetoOperator
	HTLC             ports.HTLCContractPort // optional — nil disables on-chain coordination
	Relay            ports.InteroperabilityPort
	Fiat             ports.FiatTokenPort           // optional — nil disables fCeBM operations
	EscrowRepo       ports.EscrowRepository        // optional — nil disables escrow flow
	FXAgreementBesu  ports.FXAgreementContractPort // optional — nil disables Besu FXAgreement path
	FXAgreementPente ports.FXAgreementContractPort // optional — nil disables Pente FXAgreement path
	FXRepo           ports.FXAgreementRepository   // optional — nil falls back to in-memory map (dev)
	// HTLCRepo is required in production for durable HTLC state across restarts.
	// Pass nil only in unit tests that do not need DB persistence.
	HTLCRepo   ports.HTLCRepository
	Pente      ports.PenteClientPort // optional — nil disables bilateral private context integration
	// FXContextsFile is the path to a JSON file of bilateral FX contexts (group/contract per
	// bank), written by the toolkit's deploy-fxa. Empty disables file-based resolution (falls
	// back to the Pente client). See PLAN.md "A6".
	FXContextsFile string
	RateTolPct     float64 // rate tolerance fraction, default 0.001 (0.1%)
	// CrossSpokeMode gates SettleHTLC on CounterpartyLocked. Set this to true
	// whenever the interoperability relay is active (i.e. in all production
	// deployments). When false (dev/single-spoke), the initiator can settle
	// immediately without waiting for the counterparty leg to be confirmed.
	// Do NOT use the presence of FXRepo or HTLC adapters as a proxy for this
	// flag — those are independent configuration axes.
	CrossSpokeMode  bool
	StrictHTLC      bool   // strict Agreement-HTLC enforcement mode
	SpokePrefix     string // e.g. "spoke-a" — empty disables receiver locality check
	PaladinIdentity string // full identity, e.g. "funded_operator@spoke-a-bank-a"
	Logger          *slog.Logger
}

// New builds a configured gRPC server with all payment-orchestrator handlers.
// The returned StartRelayWorkers function must be called (in a goroutine) after
// the gRPC server is listening to activate cross-spoke HTLC relay automation.
// If HTLCRepo is configured, non-terminal HTLCs are loaded from the database on
// startup; any error is returned to the caller.
func New(cfg Config) (*grpc.Server, func(context.Context), error) {
	rateTol := cfg.RateTolPct
	if rateTol <= 0 {
		rateTol = 0.001 // default 0.1%
	}
	svc := &paymentOrchestratorService{
		zeto:             cfg.Zeto,
		htlc:             cfg.HTLC,
		relay:            cfg.Relay,
		fiat:             cfg.Fiat,
		escrowRepo:       cfg.EscrowRepo,
		fxAgreementBesu:  cfg.FXAgreementBesu,
		fxAgreementPente: cfg.FXAgreementPente,
		fxRepo:           cfg.FXRepo,
		htlcRepo:         cfg.HTLCRepo,
		pente:            cfg.Pente,
		fxContexts:       newFXContextStore(cfg.FXContextsFile),
		rateTolPct:       rateTol,
		crossSpokeMode:   cfg.CrossSpokeMode,
		strictHTLC:       cfg.StrictHTLC,
		spokePrefix:      cfg.SpokePrefix,
		paladinIdentity:  cfg.PaladinIdentity,
		logger:           cfg.Logger,
		htlcs:            make(map[string]*domain.HTLCRecord),
		fxAgreements:     make(map[string]*domain.FXAgreementRecord),
	}
	if err := svc.loadHTLCsFromDB(context.Background()); err != nil {
		return nil, nil, err
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPaymentOrchestratorServiceServer(grpcServer, svc)
	return grpcServer, svc.startRelayWorkers, nil
}

// generateID returns a new UUID v4 string for record IDs.
func generateID() string {
	return uuid.New().String()
}

// isLocalReceiver checks whether the receiver identity belongs to the same spoke
// as this service instance. If spokePrefix is empty, validation is skipped (permissive).
// When spokePrefix is set (production), unparseable identities are rejected (fail closed).
func (s *paymentOrchestratorService) isLocalReceiver(receiver string) bool {
	if s.spokePrefix == "" {
		return true // no spoke configured — skip validation (dev mode)
	}
	receiverSpoke := identity.SpokePrefix(receiver)
	if receiverSpoke == "" {
		return false // unknown format — reject in production (fail closed)
	}
	return receiverSpoke == s.spokePrefix
}

// loadHTLCsFromDB pre-populates the in-memory HTLC cache from the database.
// It is called once during New() when HTLCRepo is configured. Non-terminal records
// (LOCKED, SETTLING, REFUNDING) are loaded so the service can resume in-flight operations
// after a restart without losing state.
func (s *paymentOrchestratorService) loadHTLCsFromDB(ctx context.Context) error {
	if s.htlcRepo == nil {
		return nil
	}
	records, err := s.htlcRepo.ListNonTerminal(ctx)
	if err != nil {
		return fmt.Errorf("load HTLCs from DB: %w", err)
	}
	s.mu.Lock()
	for _, r := range records {
		s.htlcs[r.ContractID] = r
	}
	s.mu.Unlock()
	s.logger.Info("loaded HTLCs from DB on startup", "count", len(records))
	return nil
}

// persistHTLC writes the current in-memory HTLC state to the database.
// If HTLCRepo is not configured, this is a no-op. Errors are logged but do not
// fail the caller — the in-memory state is authoritative; the DB is a best-effort
// write-through cache.
func (s *paymentOrchestratorService) persistHTLC(ctx context.Context, contractID string) {
	if s.htlcRepo == nil {
		return
	}
	s.mu.RLock()
	r, ok := s.htlcs[contractID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	snap := *r
	s.mu.RUnlock()
	if err := s.htlcRepo.UpdateHTLC(ctx, &snap); err != nil {
		s.logger.Error("failed to persist HTLC state",
			"contract_id", contractID, "state", snap.State, "error", err)
	}
}

// --- HTLC Dual-Layer Operations ---

func (s *paymentOrchestratorService) LockHTLC(ctx context.Context, req *pb.LockHTLCRequest) (*pb.LockHTLCResponse, error) {
	if req.Receiver == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "receiver and amount are required")
	}
	if !s.isLocalReceiver(req.Receiver) {
		return nil, status.Errorf(codes.InvalidArgument,
			"receiver %q does not belong to the local spoke %q — use a local receiver for HTLC lock",
			req.Receiver, s.spokePrefix)
	}
	// Apply smart defaults
	if req.AgreementId == "" {
		req.AgreementId = newUUID()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 3600 //#nosec G115 -- unix timestamp is always positive and fits uint64; 1h — initiator must have longer timelock
	}

	// FX Agreement gate: if an agreement_id references a known FX agreement, verify it's accepted
	// and that the HTLC terms (receiver + amount) match the agreed values for this spoke leg.
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
			//#nosec G115 -- unix timestamp is always positive and fits uint64
			if fxRecord.ExpiryDate > 0 && uint64(time.Now().Unix()) > fxRecord.ExpiryDate {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement has expired")
			}
			// Enforce receiver and amount match the agreed terms for this spoke leg.
			if err := s.validateHTLCTermsAgainstAgreement(req.Receiver, req.Amount, fxRecord); err != nil {
				return nil, err
			}
			if s.hasFXAgreementClient(fxRecord) {
				copy(agreementIDBytes[:], tradeIDBytes32(fxRecord.TradeID))
			} else if s.strictHTLC {
				agreementIDBytes = s.agreementCommitmentHash(fxRecord)
			}
		}
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

	// 4. Record on-chain HTLC coordination (if adapter configured).
	// If the on-chain lock fails, roll back the Zeto lock to avoid stranded tokens.
	var htlcTxHash string
	if s.htlc != nil {
		var zetoRefBytes [32]byte
		if b := uuidToRawBytes(lockResult.TxHash); b != nil {
			copy(zetoRefBytes[:], b)
		}
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLockBytes,
			TimeLock:    req.TimeLock,
			ZetoLockRef: zetoRefBytes,
			AgreementID: agreementIDBytes,
		})
		if err != nil {
			s.logger.Error("on-chain HTLC lock failed — rolling back Zeto lock", "error", err)
			if _, unlockErr := s.zeto.TransferLocked(ctx, strings.Join(lockResult.LockedStateIDs, ","), s.paladinIdentity, req.Amount); unlockErr != nil {
				s.logger.Error("zeto transferLocked rollback also failed", "error", unlockErr)
			}
			return nil, status.Errorf(codes.Internal, "on-chain HTLC lock failed: %v", err)
		}
	}

	// 5. Store off-chain record
	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Sender:      s.paladinIdentity,
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

	if s.htlcRepo != nil {
		if err := s.htlcRepo.CreateHTLC(ctx, record); err != nil {
			return nil, status.Errorf(codes.Internal, "persist HTLC: %v", err)
		}
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
		Secret:     secret,
	}, nil
}

func (s *paymentOrchestratorService) LockHTLCWithHashLock(ctx context.Context, req *pb.LockHTLCWithHashLockRequest) (*pb.LockHTLCWithHashLockResponse, error) {
	if req.Receiver == "" || req.Amount == "" || req.HashLock == "" {
		return nil, status.Error(codes.InvalidArgument, "receiver, amount, and hash_lock are required")
	}
	if !s.isLocalReceiver(req.Receiver) {
		return nil, status.Errorf(codes.InvalidArgument,
			"receiver %q does not belong to the local spoke %q — use a local receiver for HTLC lock",
			req.Receiver, s.spokePrefix)
	}
	// Apply smart defaults
	if req.AgreementId == "" {
		req.AgreementId = newUUID()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 1800 //#nosec G115 -- unix timestamp is always positive and fits uint64; 30min — responder must have shorter timelock than initiator
	}

	// FX Agreement gate: if an agreement_id references a known FX agreement, verify it's accepted
	// and that the HTLC terms (receiver + amount) match the agreed values for this spoke leg.
	var agreementIDBytes2 [32]byte
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
			//#nosec G115 -- unix timestamp is always positive and fits uint64
			if fxRecord.ExpiryDate > 0 && uint64(time.Now().Unix()) > fxRecord.ExpiryDate {
				return nil, status.Error(codes.FailedPrecondition, "FX agreement has expired")
			}
			// Enforce receiver and amount match the agreed terms for this spoke leg.
			if err := s.validateHTLCTermsAgainstAgreement(req.Receiver, req.Amount, fxRecord); err != nil {
				return nil, err
			}
			if s.hasFXAgreementClient(fxRecord) {
				copy(agreementIDBytes2[:], tradeIDBytes32(fxRecord.TradeID))
			} else if s.strictHTLC {
				agreementIDBytes2 = s.agreementCommitmentHash(fxRecord)
			}
		}
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

	// Record on-chain HTLC coordination (if adapter configured).
	// If the on-chain lock fails, roll back the Zeto lock to avoid stranded tokens.
	var htlcTxHash string
	if s.htlc != nil {
		var hashLock32 [32]byte
		copy(hashLock32[:], hashLockBytes)
		var zetoRefBytes [32]byte
		if b := uuidToRawBytes(lockResult.TxHash); b != nil {
			copy(zetoRefBytes[:], b)
		}
		htlcTxHash, err = s.htlc.Lock(ctx, ports.HTLCLockParams{
			ContractID:  contractIDBytes,
			Receiver:    req.Receiver,
			HashLock:    hashLock32,
			TimeLock:    req.TimeLock,
			ZetoLockRef: zetoRefBytes,
			AgreementID: agreementIDBytes2,
		})
		if err != nil {
			s.logger.Error("on-chain HTLC lock failed — rolling back Zeto lock", "error", err)
			if _, unlockErr := s.zeto.TransferLocked(ctx, strings.Join(lockResult.LockedStateIDs, ","), s.paladinIdentity, req.Amount); unlockErr != nil {
				s.logger.Error("zeto transferLocked rollback also failed", "error", unlockErr)
			}
			return nil, status.Errorf(codes.Internal, "on-chain HTLC lock failed: %v", err)
		}
	}

	// Store off-chain record (no secret — only the initiator knows it)
	record := &domain.HTLCRecord{
		ContractID:  contractID,
		AgreementID: req.AgreementId,
		Sender:      s.paladinIdentity,
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

	if s.htlcRepo != nil {
		if err := s.htlcRepo.CreateHTLC(ctx, record); err != nil {
			return nil, status.Errorf(codes.Internal, "persist HTLC: %v", err)
		}
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
				if r.State == domain.HTLCStateLocked || r.State == domain.HTLCStateSettling {
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

	// Verify secret matches hashLock
	if hashLock != record.HashLock {
		s.mu.Unlock()
		return nil, status.Error(codes.InvalidArgument, "secret does not match hashLock")
	}

	// Determine whether this is a fresh attempt (LOCKED) or a retry (SETTLING).
	isRetry := record.State == domain.HTLCStateSettling

	if !isRetry {
		if record.State != domain.HTLCStateLocked {
			s.mu.Unlock()
			return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
		}
		// CounterpartyLocked guard — enforces atomicity of the two-leg FX swap.
		//
		// The initiator must not reveal the secret until the counterparty spoke has
		// locked its matching leg. If it did, it could unlock its own tokens before
		// the mirror lock exists, leaving the counterparty with no obligation.
		//
		// Each condition is load-bearing:
		//   record.AgreementID != ""  — only FX-agreement HTLCs have a counterparty leg;
		//                               standalone HTLCs can settle freely.
		//   s.crossSpokeMode          — guard is only active when the relay is running;
		//                               disabled in dev/single-spoke mode.
		//   record.Secret != ""       — only the initiator stores the secret (it generated
		//                               it). The responder leg, created by the relay via
		//                               LockHTLCWithHashLock, has no secret and is settled
		//                               by the relay — the guard does not apply to it.
		//   !record.CounterpartyLocked — the relay sets this flag (handleRelayLockEvent)
		//                               when it observes the counterparty's LogHTLCLocked
		//                               event. Once set, the initiator may settle.
		if record.AgreementID != "" && s.crossSpokeMode && record.Secret != "" && !record.CounterpartyLocked {
			s.mu.Unlock()
			return nil, status.Error(codes.FailedPrecondition,
				"counterparty spoke has not yet locked its leg — wait for relay confirmation before settling")
		}
		// Transition to SETTLING under the lock to prevent concurrent settle attempts.
		record.State = domain.HTLCStateSettling
		record.Secret = req.Secret
		record.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()

	// Settle on-chain HTLC (reveals secret via LogHTLCClaimed event).
	// When the on-chain adapter is configured, a failure here MUST block the
	// Zeto transfer — otherwise tokens move without coordination-layer approval.
	// On retry (SETTLING), the on-chain settle already succeeded — skip it.
	var htlcTxHash string
	if s.htlc != nil && !isRetry {
		var cid, sec [32]byte
		copy(cid[:], contractIDBytes(record.ContractID))
		copy(sec[:], secretBytes)
		htlcTxHash, err = s.htlc.Settle(ctx, cid, sec)
		if err != nil {
			s.logger.Error("on-chain HTLC settle failed — rolling back to LOCKED", "contract_id", record.ContractID, "error", err)
			// On-chain settle failed — secret is NOT public. Roll back to LOCKED.
			s.mu.Lock()
			record.State = domain.HTLCStateLocked
			record.UpdatedAt = time.Now().UTC()
			s.mu.Unlock()
			s.persistHTLC(ctx, record.ContractID)
			return nil, status.Errorf(codes.Internal, "on-chain HTLC settle failed: %v", err)
		}
		// Store htlcTxHash immediately so a retry can find it.
		s.mu.Lock()
		record.HTLCTxHash = htlcTxHash
		s.mu.Unlock()
	} else if isRetry {
		htlcTxHash = record.HTLCTxHash
	}

	// Transfer locked Zeto tokens to receiver
	zetoTxHash, err := s.zeto.TransferLocked(ctx, record.ZetoLockRef, record.Receiver, record.Amount)
	if err != nil {
		// On-chain settle already succeeded (secret is public) — stay in SETTLING for retry.
		s.logger.Error("zeto transferLocked failed after on-chain settle — record stays SETTLING for retry",
			"contract_id", record.ContractID, "error", err)
		return nil, status.Errorf(codes.Internal, "zeto transferLocked: %v", err)
	}

	// Both operations succeeded — now mark SETTLED (pessimistic update).
	s.mu.Lock()
	record.State = domain.HTLCStateSettled
	record.UpdatedAt = time.Now().UTC()
	record.HTLCTxHash = htlcTxHash
	record.ZetoTxHash = zetoTxHash
	s.mu.Unlock()
	s.persistHTLC(ctx, record.ContractID)

	s.logger.Info("HTLC settled", "contract_id", record.ContractID, "zeto_tx_hash", zetoTxHash, "htlc_tx_hash", htlcTxHash)

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

	if record.State == domain.HTLCStateRefunded {
		// Already refunded — idempotent success.
		s.mu.Unlock()
		return &pb.RefundHTLCResponse{
			HtlcTxHash: record.HTLCTxHash,
			ZetoTxHash: record.ZetoTxHash,
		}, nil
	}

	// Determine whether this is a retry (REFUNDING) or a fresh attempt (LOCKED).
	isRetry := record.State == domain.HTLCStateRefunding

	if !isRetry {
		if record.State != domain.HTLCStateLocked {
			s.mu.Unlock()
			return nil, status.Errorf(codes.FailedPrecondition, "HTLC %q is in state %s, expected LOCKED", req.ContractId, record.State)
		}

		//#nosec G115 -- unix timestamp is always positive and fits uint64
		if uint64(time.Now().Unix()) < record.TimeLock {
			s.mu.Unlock()
			return nil, status.Error(codes.FailedPrecondition, "time lock has not expired yet")
		}

		// Transition to REFUNDING under the lock to prevent concurrent refund attempts.
		record.State = domain.HTLCStateRefunding
		record.UpdatedAt = time.Now().UTC()
	}
	s.mu.Unlock()
	s.persistHTLC(ctx, record.ContractID)

	// Refund on-chain HTLC coordination
	var htlcTxHash string
	if s.htlc != nil && !isRetry {
		var cid [32]byte
		copy(cid[:], contractIDBytes(req.ContractId))
		var htlcErr error
		htlcTxHash, htlcErr = s.htlc.Refund(ctx, cid)
		if htlcErr != nil {
			s.logger.Warn("on-chain HTLC refund failed", "error", htlcErr)
		}
		// Store htlcTxHash even if empty so retry path has it.
		s.mu.Lock()
		record.HTLCTxHash = htlcTxHash
		s.mu.Unlock()
	} else if isRetry {
		htlcTxHash = record.HTLCTxHash
	}

	zetoTxHash, err := s.zeto.TransferLocked(ctx, record.ZetoLockRef, record.Sender, record.Amount)
	if err != nil {
		// Stay in REFUNDING for retry.
		s.logger.Error("zeto transferLocked (refund) failed — record stays REFUNDING for retry",
			"contract_id", req.ContractId, "error", err)
		return nil, status.Errorf(codes.Internal, "zeto transferLocked refund: %v", err)
	}

	// Both operations succeeded — mark REFUNDED.
	s.mu.Lock()
	record.State = domain.HTLCStateRefunded
	record.UpdatedAt = time.Now().UTC()
	record.HTLCTxHash = htlcTxHash
	record.ZetoTxHash = zetoTxHash
	s.mu.Unlock()
	s.persistHTLC(ctx, record.ContractID)

	s.logger.Info("HTLC refunded", "contract_id", req.ContractId, "zeto_tx_hash", zetoTxHash, "htlc_tx_hash", htlcTxHash)

	return &pb.RefundHTLCResponse{
		HtlcTxHash: htlcTxHash,
		ZetoTxHash: zetoTxHash,
	}, nil
}

func (s *paymentOrchestratorService) GetHTLCStatus(ctx context.Context, req *pb.GetHTLCStatusRequest) (*pb.GetHTLCStatusResponse, error) {
	if req.ContractId == "" {
		return nil, status.Error(codes.InvalidArgument, "contract_id is required")
	}

	s.mu.RLock()
	record, ok := s.htlcs[req.ContractId]
	s.mu.RUnlock()

	if !ok && s.htlcRepo != nil {
		var err error
		record, err = s.htlcRepo.GetHTLC(ctx, req.ContractId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get HTLC: %v", err)
		}
	}
	if record == nil {
		return nil, status.Errorf(codes.NotFound, "HTLC %q not found", req.ContractId)
	}

	if err := s.checkHTLCCounterparty(ctx, record.Sender, record.Receiver); err != nil {
		return nil, err
	}

	return &pb.GetHTLCStatusResponse{
		Lock: recordToProto(record),
	}, nil
}

func (s *paymentOrchestratorService) SearchHTLC(ctx context.Context, req *pb.SearchHTLCRequest) (*pb.SearchHTLCResponse, error) {
	callerIdentity := callerIdentityFromContext(ctx)

	s.mu.RLock()
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
		// When the caller provides their BankID, only return records their institution is party to.
		// Use exact segment matching to prevent prefix-collision false positives.
		if callerIdentity != "" {
			senderBank, sErr := identity.BankID(r.Sender)
			receiverBank, rErr := identity.BankID(r.Receiver)
			if sErr != nil {
				s.logger.Warn("SearchHTLC: unparseable sender identity — excluding record", "sender", r.Sender, "error", sErr)
			}
			if rErr != nil {
				s.logger.Warn("SearchHTLC: unparseable receiver identity — excluding record", "receiver", r.Receiver, "error", rErr)
			}
			if senderBank != callerIdentity && receiverBank != callerIdentity {
				continue
			}
		}
		results = append(results, recordToProto(r))
	}
	s.mu.RUnlock()

	// Fall back to DB when in-memory map is empty and a repository is configured.
	if len(results) == 0 && s.htlcRepo != nil {
		filter := ports.HTLCFilter{
			AgreementID: req.AgreementId,
			Sender:      req.Sender,
			Receiver:    req.Receiver,
			State:       req.State,
		}
		recs, err := s.htlcRepo.ListHTLCs(ctx, filter)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "search HTLC: %v", err)
		}
		for _, r := range recs {
			results = append(results, recordToProto(r))
		}
	}

	return &pb.SearchHTLCResponse{Locks: results}, nil
}

// checkHTLCCounterparty returns PermissionDenied when the caller has identified
// themselves (via x-caller-identity metadata) but is not a party to the HTLC.
// When no identity is provided the check is skipped (permissive for dev/internal callers).
// The caller identity is a bankID (e.g. "bank-a"); sender/receiver are full Paladin
// identities (e.g. "alice@spoke-a-bank-a"). Matching uses exact BankID extraction to
// prevent substring spoofing (e.g. "bank" must not match "bank-abc").
func (s *paymentOrchestratorService) checkHTLCCounterparty(ctx context.Context, sender, receiver string) error {
	callerBankID := callerIdentityFromContext(ctx)
	if callerBankID == "" {
		return nil
	}
	senderBank, sErr := identity.BankID(sender)
	receiverBank, rErr := identity.BankID(receiver)
	if sErr != nil {
		s.logger.Warn("checkHTLCCounterparty: unparseable sender identity", "sender", sender, "error", sErr)
	}
	if rErr != nil {
		s.logger.Warn("checkHTLCCounterparty: unparseable receiver identity", "receiver", receiver, "error", rErr)
	}
	if senderBank == callerBankID || receiverBank == callerBankID {
		return nil
	}
	return status.Errorf(codes.PermissionDenied, "caller is not a counterparty of this HTLC")
}

// callerIdentityFromContext extracts the Paladin identity forwarded by the API
// gateway in the x-caller-identity gRPC metadata header.
func callerIdentityFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("x-caller-identity")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
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

func (s *paymentOrchestratorService) BurnToken(ctx context.Context, req *pb.BurnTokenRequest) (*pb.BurnTokenResponse, error) {
	if req.From == "" || req.Amount == "" {
		return nil, status.Error(codes.InvalidArgument, "from and amount are required")
	}

	txHash, err := s.zeto.Burn(ctx, req.From, req.Amount)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto burn: %v", err)
	}

	return &pb.BurnTokenResponse{TxHash: txHash}, nil
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

func (s *paymentOrchestratorService) GetBalance(ctx context.Context, _ *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	balance, err := s.zeto.Balance(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "zeto balance: %v", err)
	}

	return &pb.GetBalanceResponse{Balance: balance}, nil
}

func (s *paymentOrchestratorService) GetFiatBalance(ctx context.Context, _ *pb.GetFiatBalanceRequest) (*pb.GetFiatBalanceResponse, error) {
	if s.fiat == nil {
		return nil, status.Error(codes.Unavailable, "fiat token adapter not configured")
	}

	balance, err := s.fiat.GetFiatBalance(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fiat balance: %v", err)
	}

	return &pb.GetFiatBalanceResponse{Balance: balance}, nil
}

// --- FX Agreement Operations ---

func (s *paymentOrchestratorService) ProposeFXAgreement(ctx context.Context, req *pb.ProposeFXAgreementRequest) (*pb.ProposeFXAgreementResponse, error) {
	if req.CounterpartyB == "" || req.OriginAmount == "" || req.CounterAmount == "" ||
		req.OriginCurrency == "" || req.CounterCurrency == "" || req.Rate == "" || req.ExpiryDate == 0 {
		return nil, status.Error(codes.InvalidArgument, "counterparty_b, origin_amount, counter_amount, origin_currency, counter_currency, rate, and expiry_date are required")
	}
	//#nosec G115 -- unix timestamp is always positive and fits uint64
	if req.ExpiryDate <= uint64(time.Now().Unix()) {
		return nil, status.Error(codes.InvalidArgument, "expiry_date must be in the future")
	}
	if strings.EqualFold(req.OriginCurrency, req.CounterCurrency) {
		return nil, status.Error(codes.InvalidArgument, "origin_currency and counter_currency must be different")
	}
	if req.SourceSpokeId == "" {
		return nil, status.Error(codes.InvalidArgument, "source_spoke_id is required")
	}
	if req.DestSpokeId == "" {
		return nil, status.Error(codes.InvalidArgument, "dest_spoke_id is required")
	}
	if req.SourceReceiver == "" {
		return nil, status.Error(codes.InvalidArgument, "source_receiver is required")
	}
	if req.DestReceiver == "" {
		return nil, status.Error(codes.InvalidArgument, "dest_receiver is required")
	}
	if req.SourceSpokeId == req.DestSpokeId {
		return nil, status.Error(codes.InvalidArgument, "source_spoke_id and dest_spoke_id must be different")
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

	// A normal (non-on-behalf) propose omits `originator` — it is the calling bank. Default it
	// to this orchestrator's own Paladin identity so the FX context resolves (fx-contexts.json
	// is keyed by the local bank identity) and the agreement is attributed correctly.
	if req.Originator == "" {
		req.Originator = s.paladinIdentity
	}

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
		SourceSpokeId:   req.SourceSpokeId,
		DestSpokeId:     req.DestSpokeId,
		SourceReceiver:  req.SourceReceiver,
		DestReceiver:    req.DestReceiver,
		ExpiryDate:      req.ExpiryDate,
		State:           domain.FXStateProposed,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// On-chain submission (Pente-private). When s.pente is nil the agreement is a
	// service-layer record only (dev/off-chain mode). Atomicity: the record is persisted
	// only after a successful on-chain propose, so a revert leaves no PROPOSED row and a
	// retry re-submits cleanly.
	var txHash string
	if s.pente != nil {
		if record.GroupID == "" || record.ContractAddress == "" {
			// A6: prefer the file-backed context (group + in-group FXAgreement address
			// written by the toolkit's deploy-fxa); fall back to the Pente client's group
			// scan when no context is registered yet.
			if ref, ok := s.resolveFXContext(record); ok {
				record.GroupID = ref.GroupID
				record.ContractAddress = ref.ContractAddress
			} else {
				// Which bilateral group holds this agreement depends on the leg:
				//   - source (own propose): the CB↔this-bank group → {originator=self, counterparty}.
				//   - destination (on_behalf, coordinated by the CB): the CB↔custodian group on THIS
				//     spoke → {local CB, custodian}. The originator is remote and not a member here,
				//     so resolving by {originator, counterparty} finds no group.
				fxReq := ports.PenteContextRequest{
					TradeID:      record.TradeID,
					Originator:   record.Originator,
					Counterparty: record.CounterpartyB,
				}
				if req.OnBehalf {
					fxReq.Originator = s.paladinIdentity
					fxReq.Counterparty = record.Custodian
				}
				ctxRef, err := s.pente.EnsureFXContext(ctx, fxReq)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "ensure Pente context: %v", err)
				}
				record.GroupID = ctxRef.GroupID
				record.ContractAddress = ctxRef.ContractAddress
			}
		}
		params, err := buildFXProposalParams(tradeID, req)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid FX proposal params: %v", err)
		}
		fxClient, fxCtx, ok := s.selectFXAgreementClient(ctx, record)
		if !ok {
			return nil, status.Error(codes.FailedPrecondition, "FX agreement client not configured")
		}
		if req.OnBehalf {
			txHash, err = fxClient.ProposeOnBehalf(fxCtx, params)
		} else {
			txHash, err = fxClient.Propose(fxCtx, params)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "on-chain FX propose: %v", err)
		}
		record.OnChainTxHash = txHash
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

	// Prevent self-acceptance: the originator may not accept their own proposal.
	// on_behalf flows (coordinated by the CB relay) are exempt from this check.
	if !req.OnBehalf {
		callerBankID := callerIdentityFromContext(ctx)
		if callerBankID != "" {
			originatorBank, parseErr := identity.BankID(record.Originator)
			if parseErr == nil && originatorBank == callerBankID {
				return nil, status.Error(codes.PermissionDenied, "originator cannot accept their own FX agreement — only the counterparty may accept")
			}
		}
	}

	if s.pente != nil && (record.GroupID == "" || record.ContractAddress == "") {
		ctxRef, err := s.pente.EnsureFXContext(ctx, ports.PenteContextRequest{
			TradeID:      record.TradeID,
			Originator:   record.Originator,
			Counterparty: record.CounterpartyB,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "ensure Pente context: %v", err)
		}
		record.GroupID = ctxRef.GroupID
		record.ContractAddress = ctxRef.ContractAddress
	}

	var txHash string
	if fxClient, fxCtx, ok := s.selectFXAgreementClient(ctx, record); ok {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		var err error
		if req.OnBehalf {
			txHash, err = fxClient.AcceptOnBehalf(fxCtx, tradeIDBytes)
		} else {
			txHash, err = fxClient.Accept(fxCtx, tradeIDBytes)
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
	if s.htlc != nil && s.strictHTLC && !s.hasFXAgreementClient(record) {
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

	// Prevent self-rejection: the originator may not reject their own proposal.
	// Use Cancel to withdraw a proposal. on_behalf flows are exempt.
	if !req.OnBehalf {
		callerBankID := callerIdentityFromContext(ctx)
		if callerBankID != "" {
			originatorBank, parseErr := identity.BankID(record.Originator)
			if parseErr == nil && originatorBank == callerBankID {
				return nil, status.Error(codes.PermissionDenied, "originator cannot reject their own FX agreement — use cancel to withdraw a proposal")
			}
		}
	}

	var txHash string
	if fxClient, fxCtx, ok := s.selectFXAgreementClient(ctx, record); ok {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		var err error
		if req.OnBehalf {
			txHash, err = fxClient.RejectOnBehalf(fxCtx, tradeIDBytes)
		} else {
			txHash, err = fxClient.Reject(fxCtx, tradeIDBytes)
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
	if fxClient, fxCtx, ok := s.selectFXAgreementClient(ctx, record); ok {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		var err error
		txHash, err = fxClient.Cancel(fxCtx, tradeIDBytes)
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
	if fxClient, fxCtx, ok := s.selectFXAgreementClient(ctx, record); ok {
		var tradeIDBytes [32]byte
		copy(tradeIDBytes[:], tradeIDBytes32(req.TradeId))
		var err error
		txHash, err = fxClient.Settle(fxCtx, tradeIDBytes)
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

	return &pb.GetFXAgreementResponse{
		Agreement: fxRecordToProto(record),
	}, nil
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

	// Fallback to in-memory list when repository is not configured.
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

func (s *paymentOrchestratorService) hasFXAgreementClient(record *domain.FXAgreementRecord) bool {
	if record != nil && record.ContractAddress != "" && s.fxAgreementPente != nil {
		return true
	}
	if s.fxAgreementBesu != nil {
		return true
	}
	if s.fxAgreementPente != nil {
		return true
	}
	return false
}

func (s *paymentOrchestratorService) selectFXAgreementClient(ctx context.Context, record *domain.FXAgreementRecord) (ports.FXAgreementContractPort, context.Context, bool) {
	if record != nil && record.ContractAddress != "" && s.fxAgreementPente != nil {
		target := ports.PenteFXTarget{
			GroupID:         record.GroupID,
			ContractAddress: record.ContractAddress,
		}
		return s.fxAgreementPente, ports.WithPenteFXTarget(ctx, target), true
	}
	if s.fxAgreementBesu != nil {
		return s.fxAgreementBesu, ctx, true
	}
	if s.fxAgreementPente != nil {
		return s.fxAgreementPente, ctx, true
	}
	return nil, ctx, false
}

// --- Helpers ---

// newUUID returns a new random UUID string.
func newUUID() string {
	return uuid.NewString()
}

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
		domain.HTLCStateInvalid:   pb.HTLCState_HTLC_STATE_INVALID,
		domain.HTLCStatePending:   pb.HTLCState_HTLC_STATE_PENDING,
		domain.HTLCStateLocked:    pb.HTLCState_HTLC_STATE_LOCKED,
		domain.HTLCStateSettled:   pb.HTLCState_HTLC_STATE_SETTLED,
		domain.HTLCStateRefunded:  pb.HTLCState_HTLC_STATE_REFUNDED,
		domain.HTLCStateSettling:  pb.HTLCState_HTLC_STATE_SETTLING,
		domain.HTLCStateRefunding: pb.HTLCState_HTLC_STATE_REFUNDING,
	}
	return &pb.HTLCLock{
		ContractId:         r.ContractID,
		Sender:             r.Sender,
		Receiver:           r.Receiver,
		HashLock:           r.HashLock,
		TimeLock:           r.TimeLock,
		ZetoLockRef:        r.ZetoLockRef,
		State:              stateMap[r.State],
		CounterpartyLocked: r.CounterpartyLocked,
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
		SourceSpokeId:  r.SourceSpokeId,
		DestSpokeId:    r.DestSpokeId,
		SourceReceiver: r.SourceReceiver,
		DestReceiver:   r.DestReceiver,
		Rate:           r.Rate,
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

// tradeIDBytes32 converts a trade ID string to bytes for the on-chain [32]byte parameter.
// If hex-decodable, uses the hex bytes; otherwise hashes the string.
func tradeIDBytes32(tradeID string) []byte {
	b, err := hex.DecodeString(tradeID)
	if err == nil && len(b) == 32 {
		return b
	}
	h := sha256.Sum256([]byte(tradeID))
	return h[:]
}

// tradeIDBytes is an alias used in the FX agreement gate for HTLC lock.
func tradeIDBytes(tradeID string) []byte {
	return tradeIDBytes32(tradeID)
}

func (s *paymentOrchestratorService) agreementCommitmentHash(rec *domain.FXAgreementRecord) [32]byte {
	payload := []byte(rec.TradeID + "|" + rec.OriginAmount + "|" + rec.CounterAmount + "|" + rec.Rate)
	h := gethcrypto.Keccak256Hash(payload)
	var out [32]byte
	copy(out[:], h.Bytes())
	return out
}

// validateHTLCTermsAgainstAgreement checks that the HTLC receiver and amount match
// the FX agreement terms for the current spoke leg. It is called while s.mu is held for read,
// so it must not acquire the lock itself.
//
// The local leg is identified by comparing s.spokePrefix against fx.DestSpokeId:
//   - If this server is the dest spoke → local receiver = fx.DestReceiver, amount = fx.CounterAmount
//   - Otherwise (source spoke) → local receiver = fx.SourceReceiver, amount = fx.OriginAmount
//
// If SpokePrefix is empty (dev mode) or the agreement has no receivers set, the check is skipped.
func (s *paymentOrchestratorService) validateHTLCTermsAgainstAgreement(
	receiver, amount string,
	fx *domain.FXAgreementRecord,
) error {
	if s.spokePrefix == "" {
		return nil // dev mode — skip enforcement
	}

	var expectedReceiver, expectedAmount string
	if fx.DestSpokeId == s.spokePrefix {
		// We are the dest spoke: lock targets DestReceiver, amount is CounterAmount.
		expectedReceiver = fx.DestReceiver
		expectedAmount = fx.CounterAmount
	} else {
		// We are the source spoke: lock targets SourceReceiver, amount is OriginAmount.
		expectedReceiver = fx.SourceReceiver
		expectedAmount = fx.OriginAmount
	}

	if expectedReceiver != "" && receiver != expectedReceiver {
		return status.Errorf(codes.FailedPrecondition,
			"HTLC receiver %q does not match FX agreement receiver %q for %s",
			receiver, expectedReceiver, s.spokePrefix)
	}
	if expectedAmount != "" && amount != expectedAmount {
		return status.Errorf(codes.FailedPrecondition,
			"HTLC amount %q does not match FX agreement amount %q for %s",
			amount, expectedAmount, s.spokePrefix)
	}
	return nil
}

// resolveFXPartyAddresses resolves any Paladin identity strings in the proposal request
// to their EVM addresses using ptx_resolveVerifier. Fields that are already 0x-prefixed
// addresses are left unchanged. This allows callers (and the frontend) to use Paladin
// identities (e.g. "funded_operator@spoke-a-bank-c") for all party fields.
func resolveFXPartyAddresses(ctx context.Context, req *pb.ProposeFXAgreementRequest, zeto ports.ZetoOperator) (*pb.ProposeFXAgreementRequest, error) {
	if zeto == nil {
		return req, nil // no Paladin configured — pass through (dev/test mode)
	}
	resolved := &pb.ProposeFXAgreementRequest{
		TradeId:         req.TradeId,
		CounterpartyB:   req.CounterpartyB,
		Originator:      req.Originator,
		SettlementAgent: req.SettlementAgent,
		Custodian:       req.Custodian,
		Beneficiary:     req.Beneficiary,
		OriginAmount:    req.OriginAmount,
		CounterAmount:   req.CounterAmount,
		OriginCurrency:  req.OriginCurrency,
		CounterCurrency: req.CounterCurrency,
		Rate:            req.Rate,
		ExpiryDate:      req.ExpiryDate,
		OnBehalf:        req.OnBehalf,
		SourceSpokeId:  req.SourceSpokeId,
		DestSpokeId:    req.DestSpokeId,
		SourceReceiver: req.SourceReceiver,
		DestReceiver:   req.DestReceiver,
	}

	resolve := func(field string) (string, error) {
		if field == "" || strings.HasPrefix(field, "0x") || strings.HasPrefix(field, "0X") {
			return field, nil // already an address or empty — skip
		}
		return zeto.ResolveIdentity(ctx, field)
	}

	var err error
	if resolved.Originator, err = resolve(req.Originator); err != nil {
		return nil, fmt.Errorf("originator: %w", err)
	}
	if resolved.CounterpartyB, err = resolve(req.CounterpartyB); err != nil {
		return nil, fmt.Errorf("counterparty_b: %w", err)
	}
	if resolved.SettlementAgent, err = resolve(req.SettlementAgent); err != nil {
		return nil, fmt.Errorf("settlement_agent: %w", err)
	}
	if resolved.Custodian, err = resolve(req.Custodian); err != nil {
		return nil, fmt.Errorf("custodian: %w", err)
	}
	if resolved.Beneficiary, err = resolve(req.Beneficiary); err != nil {
		return nil, fmt.Errorf("beneficiary: %w", err)
	}
	return resolved, nil
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
	// Rate is a decimal string (e.g. "660.000000"); the on-chain rate is a uint256 in
	// 1e18 fixed-point. TODO(035): confirm the on-chain rate scale against a live deploy.
	rateRat, ok := new(big.Rat).SetString(req.Rate)
	if !ok || rateRat.Sign() <= 0 {
		return ports.FXProposalParams{}, fmt.Errorf("invalid rate: %s", req.Rate)
	}
	rateScale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	rate := new(big.Int).Quo(new(big.Int).Mul(rateRat.Num(), rateScale), rateRat.Denom())

	var originCurrency, counterCurrency [32]byte
	copy(originCurrency[:], []byte(req.OriginCurrency))
	copy(counterCurrency[:], []byte(req.CounterCurrency))

	return ports.FXProposalParams{
		TradeID:         tid,
		Originator:      partyAddress(req.Originator),
		CounterpartyB:   partyAddress(req.CounterpartyB),
		SettlementAgent: partyAddress(req.SettlementAgent),
		Custodian:       partyAddress(req.Custodian),
		Beneficiary:     partyAddress(req.Beneficiary),
		OriginAmount:    originAmount,
		CounterAmount:   counterAmount,
		OriginCurrency:  originCurrency,
		CounterCurrency: counterCurrency,
		Rate:            rate,
		ExpiryDate:      new(big.Int).SetUint64(req.ExpiryDate),
		// Routing carries the Paladin identities + spoke IDs on-chain (inside the private
		// group) so the coordinating CB reads the full deal and the relay routes by identity —
		// the addresses above collapse under the shared local-dev key and cannot be reversed.
		Routing: ports.FXRouting{
			SourceSpokeID:     req.SourceSpokeId,
			DestSpokeID:       req.DestSpokeId,
			OriginatorID:      req.Originator,
			CounterpartyID:    req.CounterpartyB,
			SettlementAgentID: req.SettlementAgent,
			CustodianID:       req.Custodian,
			BeneficiaryID:     req.Beneficiary,
			SourceReceiverID:  req.SourceReceiver,
			DestReceiverID:    req.DestReceiver,
			TradeRef:          tradeID,
		},
	}, nil
}

// partyAddress converts a party reference to an EVM address for the on-chain FXAgreement.
// A hex address is used as-is; a Paladin identity (e.g. "funded_operator@node") has no address
// in this group's EVM, so a deterministic non-zero address is derived from it (sha256[12:]) so
// propose validations pass (counterpartyB != 0) and the identity stays recoverable. The Paladin
// identity remains the source of truth in the service/relay layer.
// TODO(035): confirm on-chain party-address semantics for cross-spoke parties against a live deploy.
func partyAddress(v string) common.Address {
	v = strings.TrimSpace(v)
	if v == "" {
		return common.Address{}
	}
	if a := common.HexToAddress(v); a != (common.Address{}) {
		return a
	}
	h := sha256.Sum256([]byte(v))
	return common.BytesToAddress(h[12:])
}

// --- Cross-spoke relay workers ---

// startRelayWorkers subscribes to lock and settle events from the counterparty
// spoke via the configured interoperability relay and processes them automatically.
// It must be called in a goroutine after the gRPC server begins serving.
func (s *paymentOrchestratorService) startRelayWorkers(ctx context.Context) {
	if s.relay == nil {
		s.logger.Warn("relay: no interoperability port configured — cross-spoke automation disabled")
		return
	}
	if err := s.relay.SubscribeLockEvents(ctx, s.handleRelayLockEvent); err != nil {
		s.logger.Error("relay: failed to subscribe to lock events", "error", err)
	}
	if err := s.relay.SubscribeSettleEvents(ctx, s.handleRelaySettleEvent); err != nil {
		s.logger.Error("relay: failed to subscribe to settle events", "error", err)
	}
	s.logger.Info("relay: cross-spoke workers started")
	<-ctx.Done()
}

// handleRelayLockEvent is called by the relay subscription each time a
// LogHTLCLocked event is detected on the counterparty spoke.
// It locates the matching FX agreement by the spoke-a/spoke-b receiver identity,
// then creates the local leg via LockHTLCWithHashLock.
func (s *paymentOrchestratorService) handleRelayLockEvent(proof ports.InteroperabilityProof) error {
	// If we have a local HTLC with the same hashLock but a DIFFERENT contractId, it means
	// the counterparty spoke has locked its matching leg — mark CounterpartyLocked and return.
	// If the contractId matches our own, this is a relay echo of our own lock event — skip it.
	s.mu.Lock()
	for _, r := range s.htlcs {
		if r.HashLock == proof.HashLock {
			if r.ContractID == proof.ContractID {
				// Own-lock echo from the relay — ignore.
				s.mu.Unlock()
				s.logger.Info("relay lock: ignoring own-lock echo",
					"hashLock", proof.HashLock, "contractId", r.ContractID)
				return nil
			}
			// Different contractId with same hashLock → counterparty spoke has locked
			// its leg. Setting CounterpartyLocked unblocks the SettleHTLC guard above.
			r.CounterpartyLocked = true
			r.UpdatedAt = time.Now().UTC()
			snap := *r // snapshot under lock — prevents data race on DB write below
			s.mu.Unlock()
			s.logger.Info("relay lock: counterparty leg confirmed — settlement unblocked",
				"hashLock", proof.HashLock, "localContractId", snap.ContractID, "remoteContractId", proof.ContractID)
			if s.htlcRepo != nil {
				if err := s.htlcRepo.UpdateHTLC(context.Background(), &snap); err != nil {
					s.logger.Warn("relay lock: failed to persist CounterpartyLocked",
						"contract_id", snap.ContractID, "error", err)
				}
			}
			return nil
		}
	}
	s.mu.Unlock()

	if s.fxRepo == nil {
		s.logger.Warn("relay lock: no FX repository — cannot determine local receiver", "hashLock", proof.HashLock)
		return nil
	}

	// Parse sender/receiver from proof payload.
	var payload struct {
		Sender   string `json:"sender"`
		Receiver string `json:"receiver"`
	}
	_ = json.Unmarshal(proof.ProofPayload, &payload)

	if payload.Receiver == "" {
		s.logger.Warn("relay lock: missing receiver in proof payload", "hashLock", proof.HashLock)
		return nil
	}

	// Find the accepted FX agreement where the counterparty spoke's receiver matches.
	agreements, err := s.fxRepo.ListAgreements(context.Background(), ports.FXAgreementFilter{State: domain.FXStateAccepted})
	if err != nil {
		return fmt.Errorf("relay lock: list agreements: %w", err)
	}

	var localReceiver, localAmount, agreementID string
	for _, fx := range agreements {
		if fx.DestSpokeId == s.spokePrefix {
			// We are the dest spoke. Counterparty (source) locked for SourceReceiver.
			if fx.SourceReceiver == payload.Receiver {
				localReceiver = fx.DestReceiver
				localAmount = fx.CounterAmount
				agreementID = fx.TradeID
				break
			}
		} else {
			// We are the source spoke. Counterparty (dest) locked for DestReceiver.
			if fx.DestReceiver == payload.Receiver {
				localReceiver = fx.SourceReceiver
				localAmount = fx.OriginAmount
				agreementID = fx.TradeID
				break
			}
		}
	}

	if localReceiver == "" {
		s.logger.Warn("relay lock: no matching FX agreement for counterparty receiver",
			"counterpartyReceiver", payload.Receiver, "hashLock", proof.HashLock)
		return nil
	}

	// Use a shorter timelock than the counterparty's (15-minute margin) so the
	// local leg expires first, allowing safe refund if the counterparty abandons.
	const timeLockMarginSec = 900
	localTimeLock := proof.TimeLock
	if localTimeLock > timeLockMarginSec {
		localTimeLock -= timeLockMarginSec
	}

	_, err = s.LockHTLCWithHashLock(context.Background(), &pb.LockHTLCWithHashLockRequest{
		AgreementId: agreementID,
		Receiver:    localReceiver,
		Amount:      localAmount,
		TimeLock:    localTimeLock,
		HashLock:    proof.HashLock,
	})
	if err != nil {
		return fmt.Errorf("relay lock: LockHTLCWithHashLock: %w", err)
	}
	s.logger.Info("relay lock: local HTLC leg created",
		"hashLock", proof.HashLock, "receiver", localReceiver, "agreementId", agreementID)
	return nil
}

// handleRelaySettleEvent is called by the relay subscription each time a
// LogHTLCClaimed event is detected on the counterparty spoke.
// It extracts the secret from the proof and settles the matching local HTLC.
func (s *paymentOrchestratorService) handleRelaySettleEvent(proof ports.InteroperabilityProof) error {
	// If the contractId in the proof directly matches one of our own local HTLCs,
	// this is a relay echo of our own settle event — ignore it to avoid a
	// concurrent TransferLocked race with the user-initiated settle flow.
	s.mu.RLock()
	_, isOwnEcho := s.htlcs[proof.ContractID]
	s.mu.RUnlock()
	if isOwnEcho {
		s.logger.Info("relay settle: ignoring own-settle echo", "contractId", proof.ContractID)
		return nil
	}

	var payload struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(proof.ProofPayload, &payload); err != nil || payload.Secret == "" {
		return fmt.Errorf("relay settle: missing or invalid secret in proof payload: contractId=%s", proof.ContractID)
	}

	_, err := s.SettleHTLC(context.Background(), &pb.SettleHTLCRequest{
		ContractId: proof.ContractID,
		Secret:     payload.Secret,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			s.logger.Info("relay settle: no matching local HTLC — skipping",
				"contractId", proof.ContractID)
			return nil
		}
		return fmt.Errorf("relay settle: %w", err)
	}
	s.logger.Info("relay settle: local HTLC leg settled", "contractId", proof.ContractID)
	return nil
}

// uuidToRawBytes converts a UUID string (e.g. "dc1e6c37-f676-4848-ab6a-be8ca5c56558")
// to its 16 raw bytes. Returns nil if the input is not a valid UUID.
func uuidToRawBytes(u string) []byte {
	u = strings.ReplaceAll(u, "-", "")
	b, err := hex.DecodeString(u)
	if err != nil || len(b) != 16 {
		return nil
	}
	return b
}
