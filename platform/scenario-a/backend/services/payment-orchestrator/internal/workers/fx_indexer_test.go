// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// --- fakes ---

type fakeChainReader struct {
	groups   []ports.PenteGroup
	receipts []ports.PenteReceiptRef
	logs     map[string][]ports.PenteLog          // receiptID -> logs
	agrees   map[string]*ports.PenteFXAgreementFull // tradeIDHex -> agreement
	readErr  error
}

func (f *fakeChainReader) QueryGroups(_ context.Context, _ int) ([]ports.PenteGroup, error) {
	return f.groups, nil
}
func (f *fakeChainReader) ListReceipts(_ context.Context, _ int64, _ int) ([]ports.PenteReceiptRef, error) {
	return f.receipts, nil
}
func (f *fakeChainReader) DomainReceiptLogs(_ context.Context, receiptID string) ([]ports.PenteLog, error) {
	return f.logs[receiptID], nil
}
func (f *fakeChainReader) ReadFXAgreement(_ context.Context, _ ports.PenteFXTarget, tradeIDHex string) (*ports.PenteFXAgreementFull, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.agrees[tradeIDHex], nil
}

type memRepo struct {
	byID map[string]*domain.FXAgreementRecord
}

func newMemRepo() *memRepo { return &memRepo{byID: map[string]*domain.FXAgreementRecord{}} }

func (m *memRepo) CreateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	cp := *r
	m.byID[r.TradeID] = &cp
	return nil
}
func (m *memRepo) GetAgreement(_ context.Context, id string) (*domain.FXAgreementRecord, error) {
	r, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}
func (m *memRepo) UpdateAgreement(_ context.Context, r *domain.FXAgreementRecord) error {
	cp := *r
	m.byID[r.TradeID] = &cp
	return nil
}
func (m *memRepo) ListAgreements(_ context.Context, _ ports.FXAgreementFilter) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}
func (m *memRepo) CreateAuditEvent(_ context.Context, _ *domain.FXAgreementEvent) error { return nil }
func (m *memRepo) ListAuditEvents(_ context.Context, _ string) ([]*domain.FXAgreementEvent, error) {
	return nil, nil
}
func (m *memRepo) ListExpiredNonTerminal(_ context.Context, _ int64) ([]*domain.FXAgreementRecord, error) {
	return nil, nil
}

func testIndexer(reader ports.FXChainReaderPort, repo ports.FXAgreementRepository) *FXIndexer {
	return NewFXIndexer(reader, repo, 0, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

const (
	grpSource = "0xe2b846f26f2ef677710cfbe355788a93bb80fe24"
	fxaAddr   = "0xa9f8fef0b3df9159f1443427daa79210fceb009c"
	tradeHex  = "0x0d21af41306243e90c15378ea5c47cfe25ad06d65f6b06bc23b722766f5b053d"
)

func proposedAgreement() *ports.PenteFXAgreementFull {
	return &ports.PenteFXAgreementFull{
		TradeIDHex: tradeHex, OriginAmount: "1000", CounterAmount: "5000",
		OriginCurrency: "BRL", CounterCurrency: "COP", Rate: "5000000000000000000",
		ExpiryDate: 1783012320, State: 1,
		Routing: ports.FXRouting{
			SourceSpokeID: "spoke-brl", DestSpokeID: "spoke-cop",
			OriginatorID:     "funded_operator@spoke-brl-bank-itau",
			CounterpartyID:   "funded_operator@spoke-cop-bank-bancolombia",
			CustodianID:      "funded_operator@spoke-cop-bank-bancolombia",
			BeneficiaryID:    "funded_operator@spoke-cop-bank-davivienda",
			SourceReceiverID: "funded_operator@spoke-brl-bank-bradesco",
			DestReceiverID:   "funded_operator@spoke-cop-bank-davivienda",
			TradeRef:         "45694564-0d14-450f-8b6b-a4dd495c39c3",
		},
	}
}

func readerWith(agr *ports.PenteFXAgreementFull, topic0 string) *fakeChainReader {
	return &fakeChainReader{
		groups:   []ports.PenteGroup{{ID: "0xGROUP", Name: "fx-spoke-brl-bank-itau", ContractAddress: grpSource, Members: []string{"cb", "itau"}}},
		receipts: []ports.PenteReceiptRef{{ID: "r1", Source: grpSource, Sequence: 12, TxHash: "0xtx"}},
		logs:     map[string][]ports.PenteLog{"r1": {{Address: fxaAddr, Topics: []string{topic0, tradeHex}}}},
		agrees:   map[string]*ports.PenteFXAgreementFull{tradeHex: agr},
	}
}

func TestIndexer_ProjectsProposedAgreement(t *testing.T) {
	repo := newMemRepo()
	reader := readerWith(proposedAgreement(), topicAgreementProposed)
	testIndexer(reader, repo).runOnce(context.Background())

	rec, _ := repo.GetAgreement(context.Background(), "45694564-0d14-450f-8b6b-a4dd495c39c3")
	if rec == nil {
		t.Fatal("expected agreement projected under its UUID tradeRef")
	}
	if rec.State != domain.FXStateProposed {
		t.Errorf("state = %s, want PROPOSED", rec.State)
	}
	if rec.SourceSpokeId != "spoke-brl" || rec.DestSpokeId != "spoke-cop" {
		t.Errorf("routing not projected: %+v", rec)
	}
	if rec.CounterpartyB != "funded_operator@spoke-cop-bank-bancolombia" {
		t.Errorf("counterparty identity = %q", rec.CounterpartyB)
	}
	if rec.SourceReceiver != "funded_operator@spoke-brl-bank-bradesco" ||
		rec.DestReceiver != "funded_operator@spoke-cop-bank-davivienda" {
		t.Errorf("receivers not projected: src=%q dst=%q", rec.SourceReceiver, rec.DestReceiver)
	}
	if rec.Rate != "5" {
		t.Errorf("rate = %q, want decimal 5 (from 5e18)", rec.Rate)
	}
	if rec.GroupID != "0xGROUP" || rec.ContractAddress != fxaAddr {
		t.Errorf("group/contract not set: %q %q", rec.GroupID, rec.ContractAddress)
	}
}

func TestIndexer_AdvancesStateOnLifecycleEvent(t *testing.T) {
	repo := newMemRepo()
	// First cycle: PROPOSED.
	testIndexer(readerWith(proposedAgreement(), topicAgreementProposed), repo).runOnce(context.Background())

	// Second cycle: an AgreementSettled event fires; on-chain state is now SETTLED(3).
	settled := proposedAgreement()
	settled.State = 3
	testIndexer(readerWith(settled, topicAgreementSettled), repo).runOnce(context.Background())

	rec, _ := repo.GetAgreement(context.Background(), "45694564-0d14-450f-8b6b-a4dd495c39c3")
	if rec == nil || rec.State != domain.FXStateSettled {
		t.Fatalf("state = %v, want SETTLED", rec)
	}
}

func TestIndexer_IdempotentAcrossCycles(t *testing.T) {
	repo := newMemRepo()
	reader := readerWith(proposedAgreement(), topicAgreementProposed)
	idx := testIndexer(reader, repo)
	idx.runOnce(context.Background())
	idx.runOnce(context.Background())
	idx.runOnce(context.Background())
	if len(repo.byID) != 1 {
		t.Errorf("expected exactly 1 record after repeated sweeps, got %d", len(repo.byID))
	}
}

func TestIndexer_IgnoresNonFXAndUnknownGroups(t *testing.T) {
	repo := newMemRepo()
	reader := &fakeChainReader{
		groups:   []ports.PenteGroup{{ID: "0xGROUP", ContractAddress: grpSource}},
		receipts: []ports.PenteReceiptRef{
			{ID: "r-nonfx", Source: grpSource, Sequence: 1},
			{ID: "r-othergroup", Source: "0xdeadbeef", Sequence: 2},
		},
		logs: map[string][]ports.PenteLog{
			"r-nonfx":      {{Address: fxaAddr, Topics: []string{"0xdeadbeefdeadbeef", tradeHex}}},
			"r-othergroup": {{Address: fxaAddr, Topics: []string{topicAgreementProposed, tradeHex}}},
		},
		agrees: map[string]*ports.PenteFXAgreementFull{tradeHex: proposedAgreement()},
	}
	testIndexer(reader, repo).runOnce(context.Background())
	if len(repo.byID) != 0 {
		t.Errorf("expected no records (non-FX topic + untracked group), got %d", len(repo.byID))
	}
}

func TestRateToDecimal(t *testing.T) {
	cases := map[string]string{
		"5000000000000000000": "5",
		"5500000000000000000": "5.5",
		"660000000000000000":  "0.66",
		"0":                   "0",
	}
	for in, want := range cases {
		if got := rateToDecimal(in); got != want {
			t.Errorf("rateToDecimal(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestStateFromOrdinal(t *testing.T) {
	want := map[int]domain.FXState{
		0: domain.FXStateInvalid, 1: domain.FXStateProposed, 2: domain.FXStateAccepted,
		3: domain.FXStateSettled, 4: domain.FXStateRejected, 5: domain.FXStateCancelled,
	}
	for ord, st := range want {
		if got := stateFromOrdinal(ord); got != st {
			t.Errorf("stateFromOrdinal(%d) = %s, want %s", ord, got, st)
		}
	}
}
