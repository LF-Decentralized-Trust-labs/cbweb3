// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"os"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// fxContext is one bilateral FX context: the Pente group and the in-group FXAgreement
// address for a CB↔bank pair. It is written by the toolkit (deploy-fxa) into a JSON file
// mounted into the backend, and read at runtime to resolve the on-chain target — closing the
// address-ordering gap (deploy-fxa runs after the backend starts, so the address cannot be an
// env var). See PLAN.md "A6".
type fxContext struct {
	SpokeID         string `json:"spoke_id"`
	GroupID         string `json:"group_id"`
	ContractAddress string `json:"contract_address"`
	BankIdentity    string `json:"bank_identity"`
	CBIdentity      string `json:"cb_identity"`
}

// fxContextStore resolves FX contexts from a JSON file (an array of fxContext). The file is
// (re-)read on each lookup so contexts registered after startup — the common case, since
// deploy-fxa runs in the join soft tail — are picked up without a restart.
type fxContextStore struct {
	path string
}

func newFXContextStore(path string) *fxContextStore {
	return &fxContextStore{path: path}
}

// resolve returns the first context whose BankIdentity matches one of the given identities.
// The store holds only the contexts local to this backend's Paladin (its own group for a
// bank; every CB↔bank group for a central bank), so matching by the local bank identity picks
// the right one regardless of which party (originator/custodian) is local here.
func (s *fxContextStore) resolve(identities ...string) (fxContext, bool) {
	if s == nil || s.path == "" {
		return fxContext{}, false
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fxContext{}, false
	}
	var ctxs []fxContext
	if err := json.Unmarshal(data, &ctxs); err != nil {
		return fxContext{}, false
	}
	for _, id := range identities {
		if id == "" {
			continue
		}
		for _, c := range ctxs {
			if c.BankIdentity == id {
				return c, true
			}
		}
	}
	return fxContext{}, false
}

// resolveFXContext looks up the on-chain FX context for a record from the file store, matching
// on the record's party identities. Returns false when no context is registered yet.
func (s *paymentOrchestratorService) resolveFXContext(record *domain.FXAgreementRecord) (*ports.PenteContextResult, bool) {
	c, ok := s.fxContexts.resolve(record.Originator, record.Custodian, record.CounterpartyB)
	if !ok {
		return nil, false
	}
	return &ports.PenteContextResult{GroupID: c.GroupID, ContractAddress: c.ContractAddress}, true
}
