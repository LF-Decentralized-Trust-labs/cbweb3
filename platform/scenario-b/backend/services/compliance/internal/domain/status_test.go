// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestIsValidTransition_AllowedPaths(t *testing.T) {
	tests := []struct {
		current ParticipantStatus
		next    ParticipantStatus
	}{
		{StatusPending, StatusCredentialRequested},
		{StatusPending, StatusActive},
		{StatusCredentialRequested, StatusKYCApproved},
		{StatusCredentialRequested, StatusRevoked},
		{StatusKYCApproved, StatusActive},
		{StatusKYCApproved, StatusRevoked},
		{StatusActive, StatusFrozen},
		{StatusActive, StatusRevoked},
		{StatusFrozen, StatusActive},
		{StatusFrozen, StatusRevoked},
	}
	for _, tt := range tests {
		if !IsValidTransition(tt.current, tt.next) {
			t.Errorf("expected allowed transition %q -> %q", tt.current, tt.next)
		}
	}
}

func TestIsValidTransition_DisallowedPaths(t *testing.T) {
	tests := []struct {
		current ParticipantStatus
		next    ParticipantStatus
	}{
		{StatusPending, StatusFrozen},
		{StatusActive, StatusKYCApproved},
		{StatusRevoked, StatusActive},
		{StatusPending, StatusKYCApproved},
		{StatusPending, StatusRevoked},
		{StatusCredentialRequested, StatusActive},
		{StatusCredentialRequested, StatusFrozen},
		{StatusKYCApproved, StatusCredentialRequested},
		{StatusKYCApproved, StatusFrozen},
		{StatusActive, StatusCredentialRequested},
		{StatusFrozen, StatusCredentialRequested},
		{StatusFrozen, StatusKYCApproved},
		{StatusRevoked, StatusPending},
		{StatusRevoked, StatusFrozen},
		{StatusPending, StatusPending},
		{StatusActive, StatusActive},
	}
	for _, tt := range tests {
		if IsValidTransition(tt.current, tt.next) {
			t.Errorf("expected disallowed transition %q -> %q", tt.current, tt.next)
		}
	}
}
