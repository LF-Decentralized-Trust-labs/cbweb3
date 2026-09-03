// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// FXAgreement event topic0 hashes (keccak256 of the event signatures). Any log carrying one of
// these on a group the node belongs to is an FX agreement lifecycle event; topics[1] is the
// on-chain bytes32 tradeId. These are the sole discovery source for trade ids — FXAgreement has
// no on-chain enumeration, and getAgreement requires the id.
const (
	topicAgreementProposed  = "0xb5bd119ed8c2687d0a70ed41133e00ada095638c1eeffd635118080466e9be82"
	topicAgreementAccepted  = "0x903359345f2970df3da926dea9c27556b1fa2eceefca9e1078e1ec12c0b1b6b1"
	topicAgreementRejected  = "0xc8c3fa5db3f4920d6821b165bb9129fec7d5518df00cfee74121c6205672a25a"
	topicAgreementCancelled = "0x52cc955e7c2f4833b17e867be47af27cfd725ce2911c36e23b3258e624f5282c"
	topicAgreementSettled   = "0xed110fbe207631ea777959bf1af6eb8d96cda67aac2f5d24553d1cde56df7cf1"
)

func isFXAgreementEvent(topic0 string) bool {
	switch strings.ToLower(topic0) {
	case topicAgreementProposed, topicAgreementAccepted, topicAgreementRejected,
		topicAgreementCancelled, topicAgreementSettled:
		return true
	default:
		return false
	}
}

// rateScale is 1e18: the on-chain rate is a fixed-point integer scaled by 1e18; the projected
// DB record stores the human decimal.
var rateScale = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// FXIndexer projects on-chain FX agreements from every bilateral Pente group the node belongs to
// into the fx_agreements table, so the central bank exposes an aggregate view of its banks'
// agreements for the relay. It is chain-driven: group discovery, trade discovery (via receipt
// event logs) and the full agreement (getAgreement + getRouting) all come from Paladin — no
// out-of-band context file. Runs only on the central bank's payment-orchestrator.
type FXIndexer struct {
	reader       ports.FXChainReaderPort
	repo         ports.FXAgreementRepository
	interval     time.Duration
	logger       *slog.Logger
	groupLimit   int
	receiptLimit int
}

func NewFXIndexer(reader ports.FXChainReaderPort, repo ports.FXAgreementRepository, interval time.Duration, logger *slog.Logger) *FXIndexer {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &FXIndexer{
		reader:       reader,
		repo:         repo,
		interval:     interval,
		logger:       logger,
		groupLimit:   200,
		receiptLimit: 1000,
	}
}

// Start runs the indexer loop until ctx is cancelled.
func (w *FXIndexer) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("fx indexer started", "interval", w.interval.String())
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("fx indexer stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce sweeps all groups' receipts and projects any FX agreement it finds. It rescans from the
// start each cycle (volumes are small) and relies on idempotent upserts — no cursor to drift.
func (w *FXIndexer) runOnce(ctx context.Context) {
	groups, err := w.reader.QueryGroups(ctx, w.groupLimit)
	if err != nil {
		w.logger.Error("fx indexer: query groups failed", "error", err)
		return
	}
	// Map each group's base-ledger privacy-contract address to the group: it equals the `source`
	// of every receipt produced inside that group.
	bySource := make(map[string]ports.PenteGroup, len(groups))
	for _, g := range groups {
		if g.ContractAddress != "" {
			bySource[g.ContractAddress] = g
		}
	}

	receipts, err := w.reader.ListReceipts(ctx, 0, w.receiptLimit)
	if err != nil {
		w.logger.Error("fx indexer: list receipts failed", "error", err)
		return
	}

	seen := make(map[string]bool) // dedupe (contract|tradeId) within a sweep
	for _, r := range receipts {
		grp, ok := bySource[r.Source]
		if !ok {
			continue // receipt from a group this node does not track
		}
		logs, err := w.reader.DomainReceiptLogs(ctx, r.ID)
		if err != nil {
			w.logger.Warn("fx indexer: domain receipt read failed", "receipt", r.ID, "error", err)
			continue
		}
		for _, lg := range logs {
			if len(lg.Topics) < 2 || !isFXAgreementEvent(lg.Topics[0]) {
				continue
			}
			key := lg.Address + "|" + lg.Topics[1]
			if seen[key] {
				continue
			}
			seen[key] = true
			w.indexTrade(ctx, grp, lg.Address, lg.Topics[1], r.TxHash)
		}
	}
}

// indexTrade reads the full on-chain agreement for (group, contract, tradeId) and upserts the
// projection into the repository.
func (w *FXIndexer) indexTrade(ctx context.Context, grp ports.PenteGroup, contract, tradeIDHex, txHash string) {
	full, err := w.reader.ReadFXAgreement(ctx,
		ports.PenteFXTarget{GroupID: grp.ID, ContractAddress: contract}, tradeIDHex)
	if err != nil {
		w.logger.Warn("fx indexer: read agreement failed", "group", grp.ID, "trade", tradeIDHex, "error", err)
		return
	}
	if full == nil {
		return // race: event seen but state not yet queryable
	}

	// The DB primary key is the off-chain trade reference (UUID), reused end-to-end by the relay.
	// It is recovered from on-chain routing; fall back to the hash only if a legacy agreement
	// predates the on-chain tradeRef.
	tradeID := full.Routing.TradeRef
	if tradeID == "" {
		tradeID = tradeIDHex
	}
	state := stateFromOrdinal(full.State)

	existing, err := w.repo.GetAgreement(ctx, tradeID)
	if err != nil {
		w.logger.Error("fx indexer: get agreement failed", "trade_id", tradeID, "error", err)
		return
	}
	now := time.Now().UTC()

	if existing == nil {
		rec := &domain.FXAgreementRecord{
			TradeID:         tradeID,
			Originator:      full.Routing.OriginatorID,
			CounterpartyB:   full.Routing.CounterpartyID,
			SettlementAgent: full.Routing.SettlementAgentID,
			Custodian:       full.Routing.CustodianID,
			Beneficiary:     full.Routing.BeneficiaryID,
			OriginAmount:    full.OriginAmount,
			CounterAmount:   full.CounterAmount,
			OriginCurrency:  full.OriginCurrency,
			CounterCurrency: full.CounterCurrency,
			Rate:            rateToDecimal(full.Rate),
			SourceSpokeId:   full.Routing.SourceSpokeID,
			DestSpokeId:     full.Routing.DestSpokeID,
			SourceReceiver:  full.Routing.SourceReceiverID,
			DestReceiver:    full.Routing.DestReceiverID,
			ExpiryDate:      full.ExpiryDate,
			State:           state,
			OnChainTxHash:   txHash,
			GroupID:         grp.ID,
			ContractAddress: contract,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := w.repo.CreateAgreement(ctx, rec); err != nil {
			w.logger.Error("fx indexer: create failed", "trade_id", tradeID, "error", err)
			return
		}
		w.logger.Info("fx indexer: agreement projected", "trade_id", tradeID, "state", state, "group", grp.ID)
		return
	}

	// Only write when the authoritative on-chain state advanced; terms are immutable.
	if existing.State == state {
		return
	}
	existing.State = state
	if txHash != "" {
		existing.OnChainTxHash = txHash
	}
	existing.UpdatedAt = now
	if err := w.repo.UpdateAgreement(ctx, existing); err != nil {
		w.logger.Error("fx indexer: update failed", "trade_id", tradeID, "error", err)
		return
	}
	w.logger.Info("fx indexer: agreement state advanced", "trade_id", tradeID, "state", state)
}

// stateFromOrdinal maps the on-chain AgreementState enum ordinal to the domain state.
// Enum order: INVALID(0), PROPOSED(1), ACCEPTED(2), SETTLED(3), REJECTED(4), CANCELLED(5).
func stateFromOrdinal(v int) domain.FXState {
	switch v {
	case 1:
		return domain.FXStateProposed
	case 2:
		return domain.FXStateAccepted
	case 3:
		return domain.FXStateSettled
	case 4:
		return domain.FXStateRejected
	case 5:
		return domain.FXStateCancelled
	default:
		return domain.FXStateInvalid
	}
}

// rateToDecimal converts the on-chain 1e18 fixed-point rate integer to a human decimal string
// (e.g. "5000000000000000000" -> "5"). The relay/validation re-scale it by 1e18 on the dest leg.
func rateToDecimal(wei string) string {
	n, ok := new(big.Int).SetString(strings.TrimSpace(wei), 10)
	if !ok {
		return wei
	}
	r := new(big.Rat).SetFrac(n, rateScale)
	s := r.FloatString(18)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" {
		s = "0"
	}
	return s
}
