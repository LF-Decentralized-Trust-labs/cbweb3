// SPDX-License-Identifier: Apache-2.0

// This file implements an in-memory compliance service for KYC status retrieval.
package compliance

import (
	"strings"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// Service stores and resolves KYC statuses by subject.
type Service struct {
	mu       sync.RWMutex
	statuses map[string]domain.KYCStatus
}

// NewService initializes a compliance service with optional seed statuses.
func NewService(seed map[string]domain.KYCStatus) *Service {
	statuses := make(map[string]domain.KYCStatus)
	for subject, status := range seed {
		statuses[strings.ToLower(subject)] = status
	}
	return &Service{statuses: statuses}
}

// GetStatus returns the known status for a subject or PENDING by default.
func (s *Service) GetStatus(subject string) domain.KYCStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if status, ok := s.statuses[strings.ToLower(subject)]; ok {
		return status
	}
	return domain.KYCPending
}

// SetStatus updates the status for a subject.
func (s *Service) SetStatus(subject string, status domain.KYCStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[strings.ToLower(subject)] = status
}

