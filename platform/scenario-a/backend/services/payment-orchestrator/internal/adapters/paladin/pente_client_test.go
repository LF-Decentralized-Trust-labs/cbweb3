// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// fakePaladin is a programmable Paladin JSON-RPC endpoint for hermetic tests.
type fakePaladin struct {
	lastMethod   string
	sendTxParams map[string]any
	callParams   map[string]any
	// programmable responses
	receiptSuccess bool
	receiptFailure string
	callResult     json.RawMessage
	callErr        *rpcError
	groups         any
}

func (f *fakePaladin) handler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.lastMethod = req.Method

	writeResult := func(result any, rerr *rpcError) {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result, "error": rerr})
	}

	switch req.Method {
	case "pgroup_sendTransaction":
		_ = json.Unmarshal(req.Params[0], &f.sendTxParams)
		writeResult("tx-uuid-123", nil)
	case "ptx_getTransactionFull":
		writeResult(map[string]any{"receipt": map[string]any{
			"success": f.receiptSuccess, "failureMessage": f.receiptFailure, "transactionHash": "0xdeadbeef",
		}}, nil)
	case "pgroup_call":
		_ = json.Unmarshal(req.Params[0], &f.callParams)
		if f.callErr != nil {
			writeResult(nil, f.callErr)
			return
		}
		writeResult(f.callResult, nil)
	case "pgroup_queryGroups":
		writeResult(f.groups, nil)
	default:
		writeResult(nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method})
	}
}

func newTestClient(t *testing.T, f *fakePaladin) (*PenteClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	c := NewPenteClient(PenteClientConfig{
		BaseURL:            srv.URL,
		FXAgreementAddress: "0xC0nTract",
		Identity:           "funded_operator@spoke-brl-cb",
	})
	c.receiptInterval = time.Millisecond // keep the poll fast
	c.receiptTimeout = 2 * time.Second
	return c, srv.Close
}

func sampleParams() ports.FXProposalParams {
	var trade, cur1, cur2 [32]byte
	copy(trade[:], []byte("trade-xyz"))
	copy(cur1[:], []byte("BRL"))
	copy(cur2[:], []byte("COP"))
	return ports.FXProposalParams{
		TradeID:         trade,
		Originator:      common.HexToAddress("0x1111111111111111111111111111111111111111"),
		CounterpartyB:   common.HexToAddress("0x2222222222222222222222222222222222222222"),
		SettlementAgent: common.HexToAddress("0x3333333333333333333333333333333333333333"),
		Custodian:       common.HexToAddress("0x4444444444444444444444444444444444444444"),
		Beneficiary:     common.HexToAddress("0x5555555555555555555555555555555555555555"),
		OriginAmount:    big.NewInt(100),
		CounterAmount:   big.NewInt(66000),
		OriginCurrency:  cur1,
		CounterCurrency: cur2,
		Rate:            big.NewInt(660),
		ExpiryDate:      big.NewInt(1900000000),
	}
}

func proposeCtx() context.Context {
	return ports.WithPenteFXTarget(context.Background(), ports.PenteFXTarget{
		GroupID:         "0xGROUP",
		ContractAddress: "0xC0nTract",
	})
}

func TestPropose_SubmitsPgroupSendTransaction(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()

	txHash, err := c.Propose(proposeCtx(), sampleParams())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if txHash != "0xdeadbeef" {
		t.Errorf("txHash = %q, want the receipt transactionHash 0xdeadbeef", txHash)
	}
	// Envelope routing.
	if f.sendTxParams["domain"] != "pente" {
		t.Errorf("domain = %v, want pente", f.sendTxParams["domain"])
	}
	if f.sendTxParams["group"] != "0xGROUP" {
		t.Errorf("group = %v, want 0xGROUP", f.sendTxParams["group"])
	}
	if f.sendTxParams["from"] != "funded_operator@spoke-brl-cb" {
		t.Errorf("from = %v, want the node identity", f.sendTxParams["from"])
	}
	if !strings.EqualFold(f.sendTxParams["to"].(string), "0xc0ntract") {
		t.Errorf("to = %v, want the contract address", f.sendTxParams["to"])
	}
	fn := f.sendTxParams["function"].(map[string]any)
	if fn["name"] != "propose" {
		t.Errorf("function.name = %v, want propose", fn["name"])
	}
	in := f.sendTxParams["input"].(map[string]any)
	if in["originAmount"] != "100" || in["counterAmount"] != "66000" || in["rate"] != "660" {
		t.Errorf("amounts/rate not encoded as decimal strings: %v", in)
	}
	if _, hasOriginator := in["originator"]; hasOriginator {
		t.Errorf("propose() must NOT carry originator (that is proposeOnBehalf)")
	}
	if !strings.HasPrefix(in["tradeId"].(string), "0x") {
		t.Errorf("tradeId must be hex bytes32, got %v", in["tradeId"])
	}
}

func TestProposeOnBehalf_CarriesOriginator(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()

	if _, err := c.ProposeOnBehalf(proposeCtx(), sampleParams()); err != nil {
		t.Fatalf("ProposeOnBehalf: %v", err)
	}
	fn := f.sendTxParams["function"].(map[string]any)
	if fn["name"] != "proposeOnBehalf" {
		t.Errorf("function.name = %v, want proposeOnBehalf", fn["name"])
	}
	in := f.sendTxParams["input"].(map[string]any)
	if in["originator"] == nil {
		t.Errorf("proposeOnBehalf must carry originator")
	}
}

func TestPropose_RevertSurfacesError(t *testing.T) {
	f := &fakePaladin{receiptSuccess: false, receiptFailure: "transaction reverted: canTransact"}
	c, done := newTestClient(t, f)
	defer done()

	_, err := c.Propose(proposeCtx(), sampleParams())
	if err == nil || !strings.Contains(err.Error(), "reverted") {
		t.Fatalf("expected revert error, got %v", err)
	}
}

func TestSettle_UsesTradeIdOnly(t *testing.T) {
	f := &fakePaladin{receiptSuccess: true}
	c, done := newTestClient(t, f)
	defer done()

	var trade [32]byte
	copy(trade[:], []byte("t"))
	if _, err := c.Settle(proposeCtx(), trade); err != nil {
		t.Fatalf("Settle: %v", err)
	}
	fn := f.sendTxParams["function"].(map[string]any)
	if fn["name"] != "settle" {
		t.Errorf("function.name = %v, want settle", fn["name"])
	}
	in := f.sendTxParams["input"].(map[string]any)
	if len(in) != 1 || in["tradeId"] == nil {
		t.Errorf("settle input must be tradeId-only, got %v", in)
	}
}

func TestGetFXAgreement_DecodesState(t *testing.T) {
	f := &fakePaladin{callResult: json.RawMessage(`{"tradeId":"0xabc","state":2}`)}
	c, done := newTestClient(t, f)
	defer done()

	st, err := c.GetFXAgreement(proposeCtx(), "0xabc")
	if err != nil {
		t.Fatalf("GetFXAgreement: %v", err)
	}
	if st == nil || st.State != "FX_STATE_ACCEPTED" {
		t.Errorf("state = %+v, want FX_STATE_ACCEPTED", st)
	}
	if f.callParams["to"] == nil || f.callParams["group"] != "0xGROUP" {
		t.Errorf("call routed incorrectly: %v", f.callParams)
	}
}

func TestGetFXAgreement_NotFoundReturnsNil(t *testing.T) {
	f := &fakePaladin{callErr: &rpcError{Code: -32603,
		Message: "DOMAIN pente returned error: transaction reverted: Optional[0xe4231efa]"}}
	c, done := newTestClient(t, f)
	defer done()

	st, err := c.GetFXAgreement(proposeCtx(), "0xabc")
	if err != nil {
		t.Fatalf("not-found should be (nil,nil), got err %v", err)
	}
	if st != nil {
		t.Errorf("expected nil state for not-found, got %+v", st)
	}
}

func TestEnsureFXContext_ResolvesGroupByMembers(t *testing.T) {
	f := &fakePaladin{groups: []map[string]any{
		{"id": "0xOTHER", "members": []string{"x@n", "y@n"}},
		{"id": "0xMATCH", "members": []string{"cb@brl", "itau@brl"}},
	}}
	c, done := newTestClient(t, f)
	defer done()

	res, err := c.EnsureFXContext(context.Background(), ports.PenteContextRequest{
		Originator: "itau@brl", Counterparty: "cb@brl",
	})
	if err != nil {
		t.Fatalf("EnsureFXContext: %v", err)
	}
	if res.GroupID != "0xMATCH" {
		t.Errorf("GroupID = %q, want 0xMATCH", res.GroupID)
	}
	if !strings.EqualFold(res.ContractAddress, "0xc0ntract") {
		t.Errorf("ContractAddress = %q, want configured default", res.ContractAddress)
	}
}
