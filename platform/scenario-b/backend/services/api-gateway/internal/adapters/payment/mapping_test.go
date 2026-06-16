// SPDX-License-Identifier: Apache-2.0

package payment

import (
	"testing"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

func TestFXAgreementToResult_Nil(t *testing.T) {
	if got := fxAgreementToResult(nil); got == nil || got.TradeID != "" {
		t.Fatalf("expected empty result for nil, got %+v", got)
	}
}

func TestFXAgreementToResult_Populated(t *testing.T) {
	ag := &pb.FXAgreement{
		TradeId:         "trade-1",
		Originator:      "bank-a",
		CounterpartyB:   "bank-b",
		OriginAmount:    "100",
		CounterAmount:   "95",
		OriginCurrency:  "BRL",
		CounterCurrency: "ARS",
		Rate:            "0.95",
		ContractAddress: "0xc",
	}
	got := fxAgreementToResult(ag)
	if got.TradeID != "trade-1" || got.Originator != "bank-a" || got.CounterCurrency != "ARS" {
		t.Fatalf("unexpected mapping: %+v", got)
	}
	if got.State == "" {
		t.Fatal("expected state string to be set")
	}
}

func TestFXAgreementEventToResult_Nil(t *testing.T) {
	if got := fxAgreementEventToResult(nil); got == nil || got.ID != 0 {
		t.Fatalf("expected empty result for nil, got %+v", got)
	}
}

func TestFXAgreementEventToResult_Populated(t *testing.T) {
	ev := &pb.FXAgreementEvent{
		Id:             42,
		TradeId:        "trade-1",
		Actor:          "bank-a",
		OccurredAtUnix: 1700000000,
		Notes:          "state change",
		TxHash:         "0xtx",
	}
	got := fxAgreementEventToResult(ev)
	if got.ID != 42 || got.TradeID != "trade-1" || got.OccurredAtUnix != 1700000000 {
		t.Fatalf("unexpected mapping: %+v", got)
	}
}
