// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"math/big"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"gorm.io/gorm"
)

// hubReconciliationRepository reads the records side of the Hub reconciliation.
//
// The amounts are 18-decimal base units stored as strings. They are summed in Go with big.Int
// rather than in SQL: Postgres NUMERIC would handle them, but SQLite (the repository tests)
// degrades a large CAST to a float, and a reconciliation that rounds is not a reconciliation.
// So the queries fetch rows and the arithmetic stays in one place.
type hubReconciliationRepository struct {
	db *gorm.DB
	// holder is the Hub address being reconciled; selfBankCode is this CB's own owner_bank_id,
	// whose positions are its own money rather than an obligation toward a bank.
	holder       string
	selfBankCode string
}

func newHubReconciliationRepository(db *gorm.DB, holder, selfBankCode string) *hubReconciliationRepository {
	return &hubReconciliationRepository{db: db, holder: holder, selfBankCode: selfBankCode}
}

// InFlightBridgeIns returns the bridge-ins whose minted W-token is still expected to be sitting on
// THIS CB's Hub address, with what each payment consumed in the AMM trade and already returned.
//
// The predicates are what make the row set correspond to the address, and each one excludes a
// class of row that would otherwise be counted as held when it is not:
//
//	direction = IN            a bridge-OUT is created with leg=SETTLEMENT, reaches ACTIVE and
//	                          carries this CB's own W-token too, so the other predicates cannot
//	                          tell them apart. Its tokens sit at the SOURCE CB's address.
//	owner_bank_id <> self     the CB's own lock-mints are its own money, typically deployed into
//	                          an AMM pool, not an obligation toward a bank. They leave the address
//	                          without any record on the position.
//	mint_to_hub_address       a bridge-in may mint to a BANK's address (the gateway that signs its
//	                          own Hub swap). Empty means the executor's default, which is this
//	                          holder — so empty and holder both count, anything else does not.
//	bridge_state = ACTIVE     before it, the mint has not happened; a settled payment stays ACTIVE
//	                          forever and simply contributes zero, which the arithmetic decides.
func (r *hubReconciliationRepository) InFlightBridgeIns(ctx context.Context, wToken string) ([]services.BridgeInContribution, error) {
	var positions []domain.BridgedAssetPosition
	q := r.db.WithContext(ctx).
		Where("mirrored_asset = ?", wToken).
		Where("direction = ?", domain.BridgeDirectionIn).
		Where("leg = ?", domain.BridgeLegSettlement).
		Where("bridge_state = ?", domain.BridgeStateActive).
		Where("mint_to_hub_address = '' OR lower(mint_to_hub_address) = lower(?)", r.holder)
	if r.selfBankCode != "" {
		q = q.Where("owner_bank_id <> ?", r.selfBankCode)
	}
	if err := q.Find(&positions).Error; err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(positions))
	for i := range positions {
		ids = append(ids, positions[i].PositionID)
	}

	// What the AMM trade actually cost, per funding position. This CB executes the trade itself
	// (the delegated Step 2), so it is its own record — no other entity has to be trusted for it.
	//
	// A row is only a cost once the trade returned: a PENDING claim is a trade in flight and a FAILED
	// one never produced a figure, and reading either as "consumed 0" is indistinguishable from
	// "not swapped yet" — which for a claim in flight it effectively is.
	consumed := map[string]string{}
	if err := r.eachChunk(ids, func(chunk []string) error {
		var swaps []domain.CrossCurrencyHubSwap
		if err := r.db.WithContext(ctx).
			Where("bridge_in_position_id IN ?", chunk).
			Where("amount_in <> ''").
			Find(&swaps).Error; err != nil {
			return err
		}
		for i := range swaps {
			consumed[swaps[i].BridgeInPositionID] = swaps[i].AmountIn
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// What has already gone back. The test is the STATE, not the tx hash: the hash is recorded as
	// a pre-confirmation intent the moment the burn is broadcast, so reading it would count value
	// as gone while it is still on the address — and a reverted burn keeps its hash forever.
	returned := map[string]*big.Int{}
	if err := r.eachChunk(ids, func(chunk []string) error {
		var residues []domain.BridgedAssetPosition
		if err := r.db.WithContext(ctx).
			Where("leg = ?", domain.BridgeLegResidue).
			Where("parent_position_id IN ?", chunk).
			Where("bridge_state IN ?", []domain.BridgeState{domain.BridgeStateBurned, domain.BridgeStateReleased}).
			Find(&residues).Error; err != nil {
			return err
		}
		for i := range residues {
			amt, ok := new(big.Int).SetString(residues[i].MirroredAmount, 10)
			if !ok {
				return fmt.Errorf("residue position %s has an unparseable mirrored_amount %q",
					residues[i].PositionID, residues[i].MirroredAmount)
			}
			parent := residues[i].ParentPositionID
			if returned[parent] == nil {
				returned[parent] = new(big.Int)
			}
			returned[parent].Add(returned[parent], amt)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	out := make([]services.BridgeInContribution, 0, len(positions))
	for i := range positions {
		p := &positions[i]
		ret := "0"
		if v := returned[p.PositionID]; v != nil {
			ret = v.String()
		}
		amountIn, known := consumed[p.PositionID]
		out = append(out, services.BridgeInContribution{
			PositionID:  p.PositionID,
			OwnerBankID: p.OwnerBankID,
			Minted:      p.MirroredAmount,
			Consumed:    amountIn,
			Returned:    ret,
			// Whether the cost is RECORDED, as opposed to zero. Empty and zero are the same string
			// here, and the difference decides between "not swapped yet" and "swapped by something
			// this CB did not record" — a bank running Step 2 locally with the CB's key, which is the
			// configuration this branch migrates away from but still supports.
			SwapCostKnown: known,
		})
	}
	return out, nil
}

// idChunkSize bounds an IN (...) list. A bridge-in never leaves ACTIVE, so this set grows with
// payment history; unbounded, it would eventually exceed the driver's parameter limit and the
// reconciliation would stop working altogether rather than degrade.
const idChunkSize = 900

func (r *hubReconciliationRepository) eachChunk(ids []string, fn func([]string) error) error {
	for start := 0; start < len(ids); start += idChunkSize {
		end := start + idChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		if err := fn(ids[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// StrandedPositions returns positions of this W-token that this CB's own relayer gave up on.
//
// INFORMATIONAL ONLY — the service does not subtract these. Most of them are already inside the
// in-flight figure: a residue leg that never burned leaves its parent still accounting for the
// same amount, so subtracting it again would drive the result negative exactly in the failure the
// report exists to name. Direction is carried so an operator can tell a stuck mint (nothing on the
// address) from a stuck burn (still on it).
//
// It cannot cover a residue whose ENQUEUE failed: no position was ever created, so this CB has no
// record of it and the amount shows up as unexplained — correctly, since from here it is money
// with no known owner. The payer's own gateway is what knows (RETURN_ESCALATED).
func (r *hubReconciliationRepository) StrandedPositions(ctx context.Context, wToken string) ([]services.StrandedItem, error) {
	var positions []domain.BridgedAssetPosition
	if err := r.db.WithContext(ctx).
		Where("mirrored_asset = ?", wToken).
		Where("bridge_state = ?", domain.BridgeStateReconciliationRequired).
		Order("position_id ASC").
		Find(&positions).Error; err != nil {
		return nil, err
	}
	out := make([]services.StrandedItem, 0, len(positions))
	for i := range positions {
		p := &positions[i]
		out = append(out, services.StrandedItem{
			PositionID:  p.PositionID,
			OwnerBankID: p.OwnerBankID,
			Leg:         string(p.Leg),
			Direction:   string(p.Direction),
			Amount:      p.MirroredAmount,
			BridgeState: string(p.BridgeState),
		})
	}
	return out, nil
}
