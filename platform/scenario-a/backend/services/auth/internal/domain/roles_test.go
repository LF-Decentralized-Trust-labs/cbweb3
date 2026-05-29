package domain

import "testing"

func TestIsValidTransition_AllowedPaths(t *testing.T) {
	tests := []struct {
		current ParticipantStatus
		next    ParticipantStatus
	}{
		{ParticipantStatusPending, ParticipantStatusCredentialRequested},
		{ParticipantStatusPending, ParticipantStatusActive},
		{ParticipantStatusCredentialRequested, ParticipantStatusKYCApproved},
		{ParticipantStatusCredentialRequested, ParticipantStatusRevoked},
		{ParticipantStatusKYCApproved, ParticipantStatusActive},
		{ParticipantStatusKYCApproved, ParticipantStatusRevoked},
		{ParticipantStatusActive, ParticipantStatusFrozen},
		{ParticipantStatusActive, ParticipantStatusRevoked},
		{ParticipantStatusFrozen, ParticipantStatusActive},
		{ParticipantStatusFrozen, ParticipantStatusRevoked},
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
		{ParticipantStatusPending, ParticipantStatusFrozen},
		{ParticipantStatusActive, ParticipantStatusKYCApproved},
		{ParticipantStatusRevoked, ParticipantStatusActive},
		{ParticipantStatusPending, ParticipantStatusKYCApproved},
		{ParticipantStatusPending, ParticipantStatusRevoked},
		{ParticipantStatusCredentialRequested, ParticipantStatusActive},
		{ParticipantStatusCredentialRequested, ParticipantStatusFrozen},
		{ParticipantStatusKYCApproved, ParticipantStatusCredentialRequested},
		{ParticipantStatusKYCApproved, ParticipantStatusFrozen},
		{ParticipantStatusActive, ParticipantStatusCredentialRequested},
		{ParticipantStatusFrozen, ParticipantStatusCredentialRequested},
		{ParticipantStatusFrozen, ParticipantStatusKYCApproved},
		{ParticipantStatusRevoked, ParticipantStatusPending},
		{ParticipantStatusRevoked, ParticipantStatusFrozen},
		{ParticipantStatusPending, ParticipantStatusPending},
		{ParticipantStatusActive, ParticipantStatusActive},
	}
	for _, tt := range tests {
		if IsValidTransition(tt.current, tt.next) {
			t.Errorf("expected disallowed transition %q -> %q", tt.current, tt.next)
		}
	}
}
