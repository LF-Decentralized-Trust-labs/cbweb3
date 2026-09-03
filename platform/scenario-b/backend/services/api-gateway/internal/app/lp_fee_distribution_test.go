// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// --- Stub LP fee event repository ---

type stubLPFeeEventRepo struct {
	events []apidomain.LPFeeEvent
}

func (r *stubLPFeeEventRepo) Create(_ context.Context, event *apidomain.LPFeeEvent) error {
	r.events = append(r.events, *event)
	return nil
}

func (r *stubLPFeeEventRepo) FindBySwapOrderID(_ context.Context, swapOrderID string) (*apidomain.LPFeeEvent, error) {
	for _, e := range r.events {
		if e.SwapOrderID == swapOrderID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *stubLPFeeEventRepo) SumFeesByLPID(_ context.Context, lpID string, poolPair string) (string, error) {
	total := int64(0)
	for _, e := range r.events {
		if e.PoolPair != poolPair {
			continue
		}
		var dist map[string]map[string]string
		if err := json.Unmarshal([]byte(e.Distribution), &dist); err != nil {
			continue
		}
		if entry, ok := dist[lpID]; ok {
			var feeA int64
			fmt.Sscan(entry["fee_a"], &feeA)
			total += feeA
		}
	}
	return fmt.Sprintf("%d", total), nil
}

// buildDistribution creates a fee distribution JSON string.
func buildDistribution(shares map[string]float64, totalFeeA, totalFeeB int64) string {
	dist := map[string]map[string]string{}
	for lpID, pct := range shares {
		feeA := int64(float64(totalFeeA) * pct / 100.0)
		feeB := int64(float64(totalFeeB) * pct / 100.0)
		dist[lpID] = map[string]string{
			"fee_a": fmt.Sprintf("%d", feeA),
			"fee_b": fmt.Sprintf("%d", feeB),
		}
	}
	b, _ := json.Marshal(dist)
	return string(b)
}

// --- Tests ---

func TestFeeDistribution_twoLPs(t *testing.T) {
	// Two LPs with 50% shares each; fee of 100 token units each side
	repo := &stubLPFeeEventRepo{}

	event := apidomain.LPFeeEvent{
		EventID:      "evt-1",
		PoolPair:     "BRL/USD",
		SwapOrderID:  "swap-1",
		FeeAmountA:   "100",
		FeeAmountB:   "100",
		Distribution: buildDistribution(map[string]float64{"lp1": 50, "lp2": 50}, 100, 100),
		CreatedAt:    time.Now(),
	}
	_ = repo.Create(context.Background(), &event)

	sumLP1, err := repo.SumFeesByLPID(context.Background(), "lp1", "BRL/USD")
	if err != nil {
		t.Fatal(err)
	}
	sumLP2, _ := repo.SumFeesByLPID(context.Background(), "lp2", "BRL/USD")

	if sumLP1 != "50" {
		t.Errorf("lp1 expected 50 fee_a, got %s", sumLP1)
	}
	if sumLP2 != "50" {
		t.Errorf("lp2 expected 50 fee_a, got %s", sumLP2)
	}
}

func TestFeeDistribution_lateEntrant(t *testing.T) {
	// LP3 joins after the first swap — shares only in second event
	repo := &stubLPFeeEventRepo{}

	evt1 := apidomain.LPFeeEvent{
		EventID:      "evt-1",
		PoolPair:     "BRL/USD",
		SwapOrderID:  "swap-1",
		FeeAmountA:   "100",
		FeeAmountB:   "100",
		Distribution: buildDistribution(map[string]float64{"lp1": 100}, 100, 100),
		CreatedAt:    time.Now(),
	}
	evt2 := apidomain.LPFeeEvent{
		EventID:      "evt-2",
		PoolPair:     "BRL/USD",
		SwapOrderID:  "swap-2",
		FeeAmountA:   "100",
		FeeAmountB:   "100",
		Distribution: buildDistribution(map[string]float64{"lp1": 60, "lp3": 40}, 100, 100),
		CreatedAt:    time.Now(),
	}
	repo.Create(context.Background(), &evt1)
	repo.Create(context.Background(), &evt2)

	sumLP1, _ := repo.SumFeesByLPID(context.Background(), "lp1", "BRL/USD")
	sumLP3, _ := repo.SumFeesByLPID(context.Background(), "lp3", "BRL/USD")

	// lp1: 100 + 60 = 160
	if sumLP1 != "160" {
		t.Errorf("lp1 expected 160, got %s", sumLP1)
	}
	// lp3: only in second event = 40
	if sumLP3 != "40" {
		t.Errorf("lp3 expected 40, got %s", sumLP3)
	}
}

func TestFeeDistribution_zeroFeeBps(t *testing.T) {
	// With feeBps=0 there are no fees; distribution should be empty or zero amounts
	repo := &stubLPFeeEventRepo{}

	event := apidomain.LPFeeEvent{
		EventID:      "evt-zero",
		PoolPair:     "BRL/USD",
		SwapOrderID:  "swap-zero",
		FeeAmountA:   "0",
		FeeAmountB:   "0",
		Distribution: buildDistribution(map[string]float64{"lp1": 100}, 0, 0),
		CreatedAt:    time.Now(),
	}
	repo.Create(context.Background(), &event)

	sum, _ := repo.SumFeesByLPID(context.Background(), "lp1", "BRL/USD")
	if sum != "0" {
		t.Errorf("expected zero fee, got %s", sum)
	}
}

// Ensure the stub satisfies the Create/FindBySwapOrderID methods used in tests
var _ interface {
	Create(context.Context, *apidomain.LPFeeEvent) error
	FindBySwapOrderID(context.Context, string) (*apidomain.LPFeeEvent, error)
} = (*stubLPFeeEventRepo)(nil)
