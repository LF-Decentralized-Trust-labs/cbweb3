// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// fakeChainReader is an FXChainReaderPort double; only QueryGroups is exercised
// by the participant-roster path, the rest are stubs.
type fakeChainReader struct {
	groups []ports.PenteGroup
	err    error
}

func (f *fakeChainReader) QueryGroups(context.Context, int) ([]ports.PenteGroup, error) {
	return f.groups, f.err
}
func (f *fakeChainReader) ListReceipts(context.Context, int64, int) ([]ports.PenteReceiptRef, error) {
	return nil, nil
}
func (f *fakeChainReader) DomainReceiptLogs(context.Context, string) ([]ports.PenteLog, error) {
	return nil, nil
}
func (f *fakeChainReader) ReadFXAgreement(context.Context, ports.PenteFXTarget, string) (*ports.PenteFXAgreementFull, error) {
	return nil, nil
}

func TestListParticipantIdentities_UnionDedupedSorted(t *testing.T) {
	reader := &fakeChainReader{groups: []ports.PenteGroup{
		{ID: "g1", Members: []string{"itau@brl", "cb@brl"}},
		{ID: "g2", Members: []string{"cb@brl", "bradesco@brl"}}, // cb@brl repeats
		{ID: "g3", Members: []string{" ", ""}},                  // blanks ignored
	}}
	env := setupFXChainEnv(t, server.Config{FXChainReader: reader})

	resp, err := env.client.ListParticipantIdentities(context.Background(), &pb.ListParticipantIdentitiesRequest{})
	if err != nil {
		t.Fatalf("ListParticipantIdentities: %v", err)
	}
	want := []string{"bradesco@brl", "cb@brl", "itau@brl"} // sorted, deduped
	if len(resp.Identities) != len(want) {
		t.Fatalf("identities = %v, want %v", resp.Identities, want)
	}
	for i, id := range want {
		if resp.Identities[i] != id {
			t.Errorf("identities[%d] = %q, want %q", i, resp.Identities[i], id)
		}
	}
}

func TestListParticipantIdentities_NoReaderReturnsEmpty(t *testing.T) {
	// Pente disabled (no chain reader) → empty roster, not an error.
	env := setupFXChainEnv(t, server.Config{})

	resp, err := env.client.ListParticipantIdentities(context.Background(), &pb.ListParticipantIdentitiesRequest{})
	if err != nil {
		t.Fatalf("ListParticipantIdentities: %v", err)
	}
	if len(resp.Identities) != 0 {
		t.Errorf("identities = %v, want empty when no chain reader configured", resp.Identities)
	}
}
