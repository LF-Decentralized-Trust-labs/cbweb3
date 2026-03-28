// Package cacti provides interoperability layer implementations.
// StubRelay is a no-op/in-memory implementation for development and testing.
// A real Cacti integration will replace it once the Cacti Relayer is deployed.
package cacti

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// StubRelay is an in-memory implementation of InteroperabilityPort.
// Lock and settle events can be injected manually via InjectLockEvent / InjectSettleEvent
// for testing the full HTLC flow without a real Cacti deployment.
type StubRelay struct {
	mu              sync.RWMutex
	lockHandlers    []func(ports.InteroperabilityProof) error
	settleHandlers  []func(ports.InteroperabilityProof) error
	relayedProofs   []ports.InteroperabilityProof
	verifiedProofs  map[string]bool
	logger          *slog.Logger
}

// NewStubRelay creates a StubRelay with default settings.
func NewStubRelay(logger *slog.Logger) *StubRelay {
	return &StubRelay{
		verifiedProofs: make(map[string]bool),
		logger:         logger,
	}
}

func (s *StubRelay) SubscribeLockEvents(_ context.Context, handler func(ports.InteroperabilityProof) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockHandlers = append(s.lockHandlers, handler)
	s.logger.Info("stub: subscribed to lock events")
	return nil
}

func (s *StubRelay) SubscribeSettleEvents(_ context.Context, handler func(ports.InteroperabilityProof) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settleHandlers = append(s.settleHandlers, handler)
	s.logger.Info("stub: subscribed to settle events")
	return nil
}

func (s *StubRelay) RelayProof(_ context.Context, proof ports.InteroperabilityProof) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relayedProofs = append(s.relayedProofs, proof)
	relayTxID := fmt.Sprintf("stub-relay-%s-%d", proof.ContractID, len(s.relayedProofs))
	s.logger.Info("stub: relayed proof", "contract_id", proof.ContractID, "relay_tx_id", relayTxID)
	return relayTxID, nil
}

func (s *StubRelay) VerifyProof(_ context.Context, proof ports.InteroperabilityProof) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ok := s.verifiedProofs[proof.ContractID]; ok {
		return true, nil
	}
	s.logger.Info("stub: proof auto-verified", "contract_id", proof.ContractID)
	return true, nil
}

// --- Test helpers (not part of the InteroperabilityPort interface) ---

// InjectLockEvent simulates receiving a LogHTLCLocked event from the counterparty spoke.
func (s *StubRelay) InjectLockEvent(proof ports.InteroperabilityProof) error {
	s.mu.RLock()
	handlers := make([]func(ports.InteroperabilityProof) error, len(s.lockHandlers))
	copy(handlers, s.lockHandlers)
	s.mu.RUnlock()

	for _, h := range handlers {
		if err := h(proof); err != nil {
			return fmt.Errorf("lock handler error: %w", err)
		}
	}
	return nil
}

// InjectSettleEvent simulates receiving a LogHTLCClaimed event from the counterparty spoke.
func (s *StubRelay) InjectSettleEvent(proof ports.InteroperabilityProof) error {
	s.mu.RLock()
	handlers := make([]func(ports.InteroperabilityProof) error, len(s.settleHandlers))
	copy(handlers, s.settleHandlers)
	s.mu.RUnlock()

	for _, h := range handlers {
		if err := h(proof); err != nil {
			return fmt.Errorf("settle handler error: %w", err)
		}
	}
	return nil
}

// RelayedProofs returns all proofs that were relayed through this stub.
func (s *StubRelay) RelayedProofs() []ports.InteroperabilityProof {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.InteroperabilityProof, len(s.relayedProofs))
	copy(out, s.relayedProofs)
	return out
}

// SetVerifiedProof marks a contract ID as verified (for testing).
func (s *StubRelay) SetVerifiedProof(contractID string, verified bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifiedProofs[contractID] = verified
}
