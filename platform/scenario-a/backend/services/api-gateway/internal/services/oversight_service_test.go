// SPDX-License-Identifier: Apache-2.0

// Package services tests the OversightService disclosure workflow (T036 / FR-034/FR-035/FR-036).
// Tests MUST fail before T040 creates the service.
package services_test

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// compile-time: DisclosureResult and OversightService must be exported from the services package.
var _ *services.DisclosureResult
var _ *services.OversightService

// TestOversightService_Types asserts that the required types exist and have expected fields.
func TestOversightService_Types(t *testing.T) {
	t.Run("DisclosureResult has required fields", func(t *testing.T) {
		r := &services.DisclosureResult{}
		if r.RequestID == "" && r.State == "" {
			// Fields exist; zero values are expected before population.
		}
	})
}

// TestOversightService_Constructor confirms NewOversightService returns non-nil with nil db (test instantiation only).
func TestOversightService_Constructor(t *testing.T) {
	svc := services.NewOversightService(nil)
	if svc == nil {
		t.Fatal("NewOversightService must return non-nil")
	}
}

// TestOversightService_ValidReasonCodes confirms the service exposes a set of valid reason codes.
func TestOversightService_ValidReasonCodes(t *testing.T) {
	codes := []string{"AML_ALERT", "CFT_INVESTIGATION", "COURT_ORDER", "REGULATORY_EXAM"}
	for _, code := range codes {
		if !services.IsValidReasonCode(code) {
			t.Errorf("expected %s to be a valid reason code", code)
		}
	}
	if services.IsValidReasonCode("INVALID_CODE") {
		t.Error("INVALID_CODE must not be a valid reason code")
	}
}
