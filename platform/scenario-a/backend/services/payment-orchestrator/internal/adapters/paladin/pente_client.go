// SPDX-License-Identifier: Apache-2.0

package paladin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var (
	_ ports.PenteClientPort         = (*PenteClient)(nil)
	_ ports.FXAgreementContractPort = (*PenteClient)(nil)
	_ ports.FXChainReaderPort       = (*PenteClient)(nil)
)

// penteDomain is Paladin's privacy-group domain name for Pente (bilateral EVM privacy).
const penteDomain = "pente"

// fxaNotFoundSelector is the 4-byte selector of FXAgreement's FXA__TradeNotFound() error,
// returned by getAgreement when the trade does not exist in the group. Used to translate a
// contract revert into a (nil, nil) not-found result rather than a hard error.
const fxaNotFoundSelector = "0xe4231efa"

// PenteClient submits FXAgreement transactions into a bilateral Pente privacy group and reads
// agreement state back, by speaking Paladin's pgroup_*/ptx_* JSON-RPC directly. It replaces an
// earlier HTTP-proxy client that targeted a gateway (/api/v1/pente/fx/...) which does not exist.
type PenteClient struct {
	rpcURL          string
	httpClient      *http.Client
	defaultContract string
	identity        string
	receiptTimeout  time.Duration
	receiptInterval time.Duration
}

// PenteClientConfig configures the Paladin JSON-RPC endpoint and the calling node identity.
type PenteClientConfig struct {
	// BaseURL is the Paladin JSON-RPC endpoint (e.g. http://host.docker.internal:31648).
	BaseURL string
	// FXAgreementAddress is the default in-group FXAgreement contract address, used when the
	// per-call context does not carry one. In-group address (not the group's base contract).
	FXAgreementAddress string
	// Identity is this node's local Paladin identity (e.g. funded_operator@spoke-brl-cb),
	// used as `from` for pgroup transactions/calls. Must be local to the node.
	Identity string
}

func NewPenteClient(cfg PenteClientConfig) *PenteClient {
	return &PenteClient{
		rpcURL:          strings.TrimRight(cfg.BaseURL, "/"),
		defaultContract: normalizeAddress(cfg.FXAgreementAddress),
		identity:        cfg.Identity,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		receiptTimeout:  30 * time.Second,
		receiptInterval: 1500 * time.Millisecond,
	}
}

func normalizeAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "0x") {
		return strings.ToLower(addr)
	}
	return "0x" + strings.ToLower(addr)
}

func bytes32ToHex(v [32]byte) string {
	return "0x" + hex.EncodeToString(v[:])
}

// ---------------------------------------------------------------------------
// JSON-RPC plumbing
// ---------------------------------------------------------------------------

// rpcError is declared in client.go (same package) and reused here.

// rpc issues a JSON-RPC call and unmarshals result into out (out may be nil to ignore it).
// A JSON-RPC error is returned as *rpcError so callers can inspect the message (e.g. a revert
// selector).
func (c *PenteClient) rpc(ctx context.Context, method string, params []any, out any) error {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	if err != nil {
		return fmt.Errorf("marshal %s: %w", method, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("unmarshal %s result: %w", method, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// FXAgreement ABI (matches contracts/src/FXAgreement.sol)
// ---------------------------------------------------------------------------

type abiParam struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Components []abiParam `json:"components,omitempty"`
}

func fxFunction(name string, inputs, outputs []abiParam) map[string]any {
	fn := map[string]any{"type": "function", "name": name, "inputs": inputs}
	if outputs != nil {
		fn["outputs"] = outputs
	}
	return fn
}

// proposeInputs is the ordered ABI input list for propose(); proposeOnBehalf() inserts
// `originator` right after `tradeId`.
func proposeInputs(onBehalf bool) []abiParam {
	in := []abiParam{{Name: "tradeId", Type: "bytes32"}}
	if onBehalf {
		in = append(in, abiParam{Name: "originator", Type: "address"})
	}
	return append(in,
		abiParam{Name: "counterpartyB", Type: "address"},
		abiParam{Name: "settlementAgent", Type: "address"},
		abiParam{Name: "custodian", Type: "address"},
		abiParam{Name: "beneficiary", Type: "address"},
		abiParam{Name: "originAmount", Type: "uint256"},
		abiParam{Name: "counterAmount", Type: "uint256"},
		abiParam{Name: "originCurrency", Type: "bytes32"},
		abiParam{Name: "counterCurrency", Type: "bytes32"},
		abiParam{Name: "rate", Type: "uint256"},
		abiParam{Name: "expiryDate", Type: "uint256"},
		abiParam{Name: "routing", Type: "tuple", Components: routingTuple},
	)
}

var tradeIdOnlyInputs = []abiParam{{Name: "tradeId", Type: "bytes32"}}

// routingTuple is the ABI component list of FXAgreementLibrary.Routing, used both as the
// `routing` input to propose/proposeOnBehalf and as the getRouting() return tuple.
var routingTuple = []abiParam{
	{Name: "sourceSpokeId", Type: "string"}, {Name: "destSpokeId", Type: "string"},
	{Name: "originatorId", Type: "string"}, {Name: "counterpartyId", Type: "string"},
	{Name: "settlementAgentId", Type: "string"}, {Name: "custodianId", Type: "string"},
	{Name: "beneficiaryId", Type: "string"}, {Name: "sourceReceiverId", Type: "string"},
	{Name: "destReceiverId", Type: "string"}, {Name: "tradeRef", Type: "string"},
}

// fxAgreementTuple is the ABI component list of FXAgreementLibrary.FxAgreement, used to decode
// getAgreement()'s return value.
var fxAgreementTuple = []abiParam{
	{Name: "tradeId", Type: "bytes32"}, {Name: "originator", Type: "address"},
	{Name: "counterpartyB", Type: "address"}, {Name: "settlementAgent", Type: "address"},
	{Name: "custodian", Type: "address"}, {Name: "beneficiary", Type: "address"},
	{Name: "originAmount", Type: "uint256"}, {Name: "counterAmount", Type: "uint256"},
	{Name: "originCurrency", Type: "bytes32"}, {Name: "counterCurrency", Type: "bytes32"},
	{Name: "rate", Type: "uint256"}, {Name: "expiryDate", Type: "uint256"},
	{Name: "state", Type: "uint8"},
}

// agreementStateName maps the on-chain AgreementState enum to the proto FX_STATE_* name.
// Enum order (FXAgreementLibrary.AgreementState): INVALID, PROPOSED, ACCEPTED, SETTLED,
// REJECTED, CANCELLED — so 3=SETTLED, 4=REJECTED, 5=CANCELLED.
func agreementStateName(v int) string {
	switch v {
	case 1:
		return "FX_STATE_PROPOSED"
	case 2:
		return "FX_STATE_ACCEPTED"
	case 3:
		return "FX_STATE_SETTLED"
	case 4:
		return "FX_STATE_REJECTED"
	case 5:
		return "FX_STATE_CANCELLED"
	default:
		return "FX_STATE_INVALID"
	}
}

// ---------------------------------------------------------------------------
// pgroup send / call
// ---------------------------------------------------------------------------

func (c *PenteClient) resolveTarget(ctx context.Context) (ports.PenteFXTarget, error) {
	if target, ok := ports.PenteFXTargetFromContext(ctx); ok {
		target.ContractAddress = normalizeAddress(target.ContractAddress)
		if target.ContractAddress != "" && target.GroupID != "" {
			return target, nil
		}
		if target.ContractAddress != "" && c.defaultContract != "" {
			return target, nil
		}
	}
	if c.defaultContract != "" {
		return ports.PenteFXTarget{ContractAddress: c.defaultContract}, nil
	}
	return ports.PenteFXTarget{}, fmt.Errorf("missing Pente FX target (group/contract) in context")
}

// sendTx submits a pgroup transaction and waits for its receipt, returning the on-chain
// transaction hash. It errors if the transaction reverts.
func (c *PenteClient) sendTx(ctx context.Context, target ports.PenteFXTarget, fn map[string]any, input map[string]any) (string, error) {
	params := []any{map[string]any{
		"domain":   penteDomain,
		"group":    target.GroupID,
		"from":     c.identity,
		"to":       target.ContractAddress,
		"function": fn,
		"input":    input,
	}}
	var txID string
	if err := c.rpc(ctx, "pgroup_sendTransaction", params, &txID); err != nil {
		return "", err
	}
	return c.waitReceipt(ctx, txID)
}

// callView executes a read-only pgroup call and unmarshals the decoded result into out.
func (c *PenteClient) callView(ctx context.Context, target ports.PenteFXTarget, fn map[string]any, input map[string]any, out any) error {
	params := []any{map[string]any{
		"domain":   penteDomain,
		"group":    target.GroupID,
		"from":     c.identity,
		"to":       target.ContractAddress,
		"function": fn,
		"input":    input,
	}}
	return c.rpc(ctx, "pgroup_call", params, out)
}

// waitReceipt polls ptx_getTransactionFull until a receipt is available, returning the
// transaction hash on success or an error describing the revert on failure.
func (c *PenteClient) waitReceipt(ctx context.Context, txID string) (string, error) {
	deadline := time.Now().Add(c.receiptTimeout)
	for {
		var full struct {
			Receipt *struct {
				Success         bool   `json:"success"`
				FailureMessage  string `json:"failureMessage"`
				TransactionHash string `json:"transactionHash"`
			} `json:"receipt"`
		}
		if err := c.rpc(ctx, "ptx_getTransactionFull", []any{txID}, &full); err != nil {
			return "", err
		}
		if full.Receipt != nil {
			if !full.Receipt.Success {
				return "", fmt.Errorf("pgroup transaction %s reverted: %s", txID, full.Receipt.FailureMessage)
			}
			if full.Receipt.TransactionHash != "" {
				return full.Receipt.TransactionHash, nil
			}
			return txID, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.receiptInterval):
			if time.Now().After(deadline) {
				return "", fmt.Errorf("timed out waiting for pgroup transaction %s receipt", txID)
			}
		}
	}
}

// proposeInput builds the ABI input map for propose/proposeOnBehalf. The on-chain party address
// fields are the Paladin-RESOLVED in-group addresses of the party identities (via
// resolvePartyAddr), NOT the sha256 placeholders: a party must submit its own lifecycle txs
// (counterpartyB → accept/reject, originator → cancel), and those are gated on
// `msg.sender == <party>`, so the stored address must equal the party's real signer key.
// Identities that don't resolve locally (remote cross-spoke parties) fall back to the placeholder
// — nobody submits as them on this spoke, so only their presence (non-zero) matters.
func (c *PenteClient) proposeInput(ctx context.Context, params ports.FXProposalParams, onBehalf bool) map[string]any {
	r := params.Routing
	in := map[string]any{
		"tradeId":         bytes32ToHex(params.TradeID),
		"counterpartyB":   c.resolvePartyAddr(ctx, r.CounterpartyID, params.CounterpartyB.Hex()),
		"settlementAgent": c.resolvePartyAddr(ctx, r.SettlementAgentID, params.SettlementAgent.Hex()),
		"custodian":       c.resolvePartyAddr(ctx, r.CustodianID, params.Custodian.Hex()),
		"beneficiary":     c.resolvePartyAddr(ctx, r.BeneficiaryID, params.Beneficiary.Hex()),
		"originAmount":    bigToString(params.OriginAmount),
		"counterAmount":   bigToString(params.CounterAmount),
		"originCurrency":  bytes32ToHex(params.OriginCurrency),
		"counterCurrency": bytes32ToHex(params.CounterCurrency),
		"rate":            bigToString(params.Rate),
		"expiryDate":      bigToString(params.ExpiryDate),
		"routing":         routingInput(r),
	}
	if onBehalf {
		in["originator"] = c.resolvePartyAddr(ctx, r.OriginatorID, params.Originator.Hex())
	}
	return in
}

// resolvePartyAddr resolves a Paladin identity to its in-group EVM address via ptx_resolveVerifier
// (the address Paladin will use as msg.sender when that identity submits a tx). Falls back to the
// provided placeholder when the identity is empty or not resolvable on this node (a remote party).
func (c *PenteClient) resolvePartyAddr(ctx context.Context, identity, fallback string) string {
	if identity == "" {
		return fallback
	}
	var addr string
	if err := c.rpc(ctx, "ptx_resolveVerifier", []any{identity, "ecdsa:secp256k1", "eth_address"}, &addr); err == nil && addr != "" {
		return addr
	}
	return fallback
}

// routingInput builds the ABI tuple map for the FXAgreementLibrary.Routing struct.
func routingInput(r ports.FXRouting) map[string]any {
	return map[string]any{
		"sourceSpokeId":     r.SourceSpokeID,
		"destSpokeId":       r.DestSpokeID,
		"originatorId":      r.OriginatorID,
		"counterpartyId":    r.CounterpartyID,
		"settlementAgentId": r.SettlementAgentID,
		"custodianId":       r.CustodianID,
		"beneficiaryId":     r.BeneficiaryID,
		"sourceReceiverId":  r.SourceReceiverID,
		"destReceiverId":    r.DestReceiverID,
		"tradeRef":          r.TradeRef,
	}
}

func bigToString(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

// ---------------------------------------------------------------------------
// FXAgreementContractPort
// ---------------------------------------------------------------------------

func (c *PenteClient) Propose(ctx context.Context, params ports.FXProposalParams) (string, error) {
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return "", err
	}
	return c.sendTx(ctx, target, fxFunction("propose", proposeInputs(false), nil), c.proposeInput(ctx, params, false))
}

func (c *PenteClient) ProposeOnBehalf(ctx context.Context, params ports.FXProposalParams) (string, error) {
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return "", err
	}
	return c.sendTx(ctx, target, fxFunction("proposeOnBehalf", proposeInputs(true), nil), c.proposeInput(ctx, params, true))
}

func (c *PenteClient) tradeIdTx(ctx context.Context, fnName string, tradeID [32]byte) (string, error) {
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return "", err
	}
	return c.sendTx(ctx, target, fxFunction(fnName, tradeIdOnlyInputs, nil),
		map[string]any{"tradeId": bytes32ToHex(tradeID)})
}

func (c *PenteClient) Accept(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "accept", tradeID)
}
func (c *PenteClient) AcceptOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "acceptOnBehalf", tradeID)
}
func (c *PenteClient) Reject(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "reject", tradeID)
}
func (c *PenteClient) RejectOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "rejectOnBehalf", tradeID)
}
func (c *PenteClient) Cancel(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "cancel", tradeID)
}
func (c *PenteClient) Settle(ctx context.Context, tradeID [32]byte) (string, error) {
	return c.tradeIdTx(ctx, "settle", tradeID)
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// GetFXAgreement reads an agreement's state from the group. tradeID is the hex-encoded bytes32.
// Returns (nil, nil) when the agreement does not exist (FXA__TradeNotFound revert).
func (c *PenteClient) GetFXAgreement(ctx context.Context, tradeID string) (*ports.PenteFXAgreementState, error) {
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return nil, err
	}
	fn := fxFunction("getAgreement", tradeIdOnlyInputs,
		[]abiParam{{Name: "", Type: "tuple", Components: fxAgreementTuple}})

	// Paladin decodes the tuple output into a JSON object keyed by component name, wrapped
	// positionally under "0"; callTuple unwraps it.
	var out struct {
		TradeID string      `json:"tradeId"`
		State   json.Number `json:"state"`
	}
	if err := c.callTuple(ctx, target, fn, map[string]any{"tradeId": tradeID}, &out); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	stateInt, _ := out.State.Int64()
	return &ports.PenteFXAgreementState{
		TradeID: out.TradeID,
		State:   agreementStateName(int(stateInt)),
	}, nil
}

func isNotFound(err error) bool {
	var re *rpcError
	if e, ok := err.(*rpcError); ok {
		re = e
	}
	if re == nil {
		return false
	}
	return strings.Contains(strings.ToLower(re.Message), fxaNotFoundSelector) ||
		strings.Contains(re.Message, "FXA__TradeNotFound")
}

// ---------------------------------------------------------------------------
// PenteClientPort — resolve the bilateral group for a CB↔bank pair
// ---------------------------------------------------------------------------

// EnsureFXContext resolves the Pente group whose members are exactly {originator, counterparty}
// (Paladin identities). The group is created by the toolkit; this only looks it up dynamically
// via pgroup_queryGroups, so banks that join later are resolved with no provisioning.
//
// The in-group FXAgreement address is taken from the configured default when present; otherwise
// it is discovered from chain (findFXAgreementInGroup) — the calling node is a group member, so
// it has the group's transaction receipts including the FXAgreement deploy. This is what lets a
// destination coordinator (CB) resolve a CB↔custodian group it did not itself provision.
func (c *PenteClient) EnsureFXContext(ctx context.Context, req ports.PenteContextRequest) (*ports.PenteContextResult, error) {
	if req.Originator == "" || req.Counterparty == "" {
		return nil, fmt.Errorf("EnsureFXContext: originator and counterparty identities are required")
	}
	var groups []struct {
		ID              string   `json:"id"`
		Members         []string `json:"members"`
		ContractAddress string   `json:"contractAddress"`
	}
	if err := c.rpc(ctx, "pgroup_queryGroups", []any{map[string]any{"limit": 200}}, &groups); err != nil {
		return nil, fmt.Errorf("query pente groups: %w", err)
	}
	want := map[string]bool{req.Originator: true, req.Counterparty: true}
	for _, g := range groups {
		if len(g.Members) != 2 {
			continue
		}
		if want[g.Members[0]] && want[g.Members[1]] && g.Members[0] != g.Members[1] {
			if c.defaultContract != "" {
				return &ports.PenteContextResult{GroupID: g.ID, ContractAddress: c.defaultContract}, nil
			}
			addr, err := c.findFXAgreementInGroup(ctx, g.ID, g.ContractAddress)
			if err != nil {
				return nil, fmt.Errorf("resolved group %s but could not locate its FXAgreement: %w", g.ID, err)
			}
			return &ports.PenteContextResult{GroupID: g.ID, ContractAddress: addr}, nil
		}
	}
	return nil, fmt.Errorf("no bilateral Pente group found for members {%s, %s}", req.Originator, req.Counterparty)
}

// findFXAgreementInGroup discovers the in-group FXAgreement contract address from chain, for a
// group the calling node is a member of. It scans the group's transaction receipts (matched by
// the group's base-ledger privacy-contract address, which equals each receipt's `source`),
// collects the distinct contract addresses touched, and returns the first that answers the
// FXAgreement-specific getRouting() call (reverting FXA__TradeNotFound for a missing trade — the
// in-group IdentityRegistry has no such function and errors differently).
func (c *PenteClient) findFXAgreementInGroup(ctx context.Context, groupID, groupSource string) (string, error) {
	src := normalizeAddress(groupSource)
	if src == "" {
		return "", fmt.Errorf("group %s has no base-ledger contract address", groupID)
	}
	receipts, err := c.ListReceipts(ctx, 0, 1000)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, r := range receipts {
		if r.Source != src {
			continue
		}
		addr, err := c.domainReceiptContractAddr(ctx, r.ID)
		if err != nil || addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		if c.isFXAgreementAt(ctx, ports.PenteFXTarget{GroupID: groupID, ContractAddress: addr}) {
			return addr, nil
		}
	}
	return "", fmt.Errorf("no FXAgreement deployed in group %s", groupID)
}

// domainReceiptContractAddr returns the contract address touched by a Pente transaction (the
// deployed address for a deploy; the `to` for a call), from its domain receipt.
func (c *PenteClient) domainReceiptContractAddr(ctx context.Context, receiptID string) (string, error) {
	var dr struct {
		Receipt struct {
			ContractAddress string `json:"contractAddress"`
		} `json:"receipt"`
	}
	if err := c.rpc(ctx, "ptx_getDomainReceipt", []any{penteDomain, receiptID}, &dr); err != nil {
		return "", err
	}
	return normalizeAddress(dr.Receipt.ContractAddress), nil
}

// isFXAgreementAt reports whether the contract at target is an FXAgreement, by probing getRouting
// for a non-existent trade: FXAgreement reverts FXA__TradeNotFound (isNotFound), whereas other
// in-group contracts (e.g. the IdentityRegistry) revert with a different/empty error.
func (c *PenteClient) isFXAgreementAt(ctx context.Context, target ports.PenteFXTarget) bool {
	fn := fxFunction("getRouting", tradeIdOnlyInputs,
		[]abiParam{{Name: "", Type: "tuple", Components: routingTuple}})
	var out any
	err := c.callTuple(ctx, target, fn, map[string]any{"tradeId": zeroTradeIDHex}, &out)
	return isNotFound(err)
}

// zeroTradeIDHex is a bytes32 trade id guaranteed absent, used to probe getRouting.
const zeroTradeIDHex = "0x0000000000000000000000000000000000000000000000000000000000000001"

// ---------------------------------------------------------------------------
// FXChainReaderPort — chain-driven read side for the CB aggregation indexer
// ---------------------------------------------------------------------------

// QueryGroups lists the bilateral privacy groups this node belongs to, including each group's
// base-ledger privacy-contract address (which equals the `source` of receipts in that group).
func (c *PenteClient) QueryGroups(ctx context.Context, limit int) ([]ports.PenteGroup, error) {
	if limit <= 0 {
		limit = 200
	}
	var raw []struct {
		ID              string   `json:"id"`
		Name            string   `json:"name"`
		Members         []string `json:"members"`
		ContractAddress string   `json:"contractAddress"`
	}
	if err := c.rpc(ctx, "pgroup_queryGroups", []any{map[string]any{"limit": limit}}, &raw); err != nil {
		return nil, fmt.Errorf("query pente groups: %w", err)
	}
	groups := make([]ports.PenteGroup, 0, len(raw))
	for _, g := range raw {
		groups = append(groups, ports.PenteGroup{
			ID: g.ID, Name: g.Name, Members: g.Members,
			ContractAddress: normalizeAddress(g.ContractAddress),
		})
	}
	return groups, nil
}

// ListReceipts returns Pente-domain transaction receipts with sequence strictly greater than
// afterSequence, in ascending sequence order (for a stable incremental cursor).
func (c *PenteClient) ListReceipts(ctx context.Context, afterSequence int64, limit int) ([]ports.PenteReceiptRef, error) {
	if limit <= 0 {
		limit = 200
	}
	query := map[string]any{
		"limit":       limit,
		"sort":        []string{"sequence"},
		"equal":       []map[string]any{{"field": "domain", "value": penteDomain}},
		"greaterThan": []map[string]any{{"field": "sequence", "value": afterSequence}},
	}
	var raw []struct {
		ID              string `json:"id"`
		Sequence        int64  `json:"sequence"`
		Source          string `json:"source"`
		TransactionHash string `json:"transactionHash"`
	}
	if err := c.rpc(ctx, "ptx_queryTransactionReceipts", []any{query}, &raw); err != nil {
		return nil, fmt.Errorf("query transaction receipts: %w", err)
	}
	out := make([]ports.PenteReceiptRef, 0, len(raw))
	for _, r := range raw {
		out = append(out, ports.PenteReceiptRef{
			ID: r.ID, Source: normalizeAddress(r.Source), Sequence: r.Sequence, TxHash: r.TransactionHash,
		})
	}
	return out, nil
}

// DomainReceiptLogs returns the decoded EVM logs of a Pente transaction's domain receipt.
// Returns (nil, nil) when the receipt carries no domain receipt (e.g. a non-EVM tx).
func (c *PenteClient) DomainReceiptLogs(ctx context.Context, receiptID string) ([]ports.PenteLog, error) {
	var dr struct {
		Receipt *struct {
			Logs []struct {
				Address string   `json:"address"`
				Topics  []string `json:"topics"`
				Data    string   `json:"data"`
			} `json:"logs"`
		} `json:"receipt"`
	}
	if err := c.rpc(ctx, "ptx_getDomainReceipt", []any{penteDomain, receiptID}, &dr); err != nil {
		return nil, err
	}
	if dr.Receipt == nil {
		return nil, nil
	}
	logs := make([]ports.PenteLog, 0, len(dr.Receipt.Logs))
	for _, l := range dr.Receipt.Logs {
		logs = append(logs, ports.PenteLog{Address: normalizeAddress(l.Address), Topics: l.Topics, Data: l.Data})
	}
	return logs, nil
}

// ReadFXAgreement reads the full agreement (getAgreement) and its routing (getRouting) for a
// trade in a group. Returns (nil, nil) when the trade does not exist.
func (c *PenteClient) ReadFXAgreement(ctx context.Context, target ports.PenteFXTarget, tradeIDHex string) (*ports.PenteFXAgreementFull, error) {
	target.ContractAddress = normalizeAddress(target.ContractAddress)

	getFn := fxFunction("getAgreement", tradeIdOnlyInputs,
		[]abiParam{{Name: "", Type: "tuple", Components: fxAgreementTuple}})
	var a struct {
		TradeID         string      `json:"tradeId"`
		Originator      string      `json:"originator"`
		CounterpartyB   string      `json:"counterpartyB"`
		SettlementAgent string      `json:"settlementAgent"`
		Custodian       string      `json:"custodian"`
		Beneficiary     string      `json:"beneficiary"`
		OriginAmount    json.Number `json:"originAmount"`
		CounterAmount   json.Number `json:"counterAmount"`
		OriginCurrency  string      `json:"originCurrency"`
		CounterCurrency string      `json:"counterCurrency"`
		Rate            json.Number `json:"rate"`
		ExpiryDate      json.Number `json:"expiryDate"`
		State           json.Number `json:"state"`
	}
	if err := c.callTuple(ctx, target, getFn, map[string]any{"tradeId": tradeIDHex}, &a); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	rtFn := fxFunction("getRouting", tradeIdOnlyInputs,
		[]abiParam{{Name: "", Type: "tuple", Components: routingTuple}})
	var rt struct {
		SourceSpokeID     string `json:"sourceSpokeId"`
		DestSpokeID       string `json:"destSpokeId"`
		OriginatorID      string `json:"originatorId"`
		CounterpartyID    string `json:"counterpartyId"`
		SettlementAgentID string `json:"settlementAgentId"`
		CustodianID       string `json:"custodianId"`
		BeneficiaryID     string `json:"beneficiaryId"`
		SourceReceiverID  string `json:"sourceReceiverId"`
		DestReceiverID    string `json:"destReceiverId"`
		TradeRef          string `json:"tradeRef"`
	}
	if err := c.callTuple(ctx, target, rtFn, map[string]any{"tradeId": tradeIDHex}, &rt); err != nil {
		if !isNotFound(err) {
			return nil, err
		}
	}

	state, _ := a.State.Int64()
	expiry, _ := a.ExpiryDate.Int64()
	return &ports.PenteFXAgreementFull{
		TradeIDHex:      a.TradeID,
		Originator:      a.Originator,
		CounterpartyB:   a.CounterpartyB,
		SettlementAgent: a.SettlementAgent,
		Custodian:       a.Custodian,
		Beneficiary:     a.Beneficiary,
		OriginAmount:    a.OriginAmount.String(),
		CounterAmount:   a.CounterAmount.String(),
		OriginCurrency:  decodeBytes32String(a.OriginCurrency),
		CounterCurrency: decodeBytes32String(a.CounterCurrency),
		Rate:            a.Rate.String(),
		//#nosec G115 -- on-chain expiryDate is a positive unix timestamp
		ExpiryDate: uint64(expiry),
		State:      int(state),
		Routing: ports.FXRouting{
			SourceSpokeID:     rt.SourceSpokeID,
			DestSpokeID:       rt.DestSpokeID,
			OriginatorID:      rt.OriginatorID,
			CounterpartyID:    rt.CounterpartyID,
			SettlementAgentID: rt.SettlementAgentID,
			CustodianID:       rt.CustodianID,
			BeneficiaryID:     rt.BeneficiaryID,
			SourceReceiverID:  rt.SourceReceiverID,
			DestReceiverID:    rt.DestReceiverID,
			TradeRef:          rt.TradeRef,
		},
	}, nil
}

// callTuple executes a read-only pgroup call whose single output is a struct/tuple. Paladin
// returns such an output positionally keyed as "0"; some code paths return it unwrapped, so this
// handles both forms.
func (c *PenteClient) callTuple(ctx context.Context, target ports.PenteFXTarget, fn map[string]any, input map[string]any, out any) error {
	var raw json.RawMessage
	if err := c.callView(ctx, target, fn, input, &raw); err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		if v, ok := m["0"]; ok {
			return json.Unmarshal(v, out)
		}
	}
	return json.Unmarshal(raw, out)
}

// decodeBytes32String decodes a hex-encoded bytes32 (e.g. "0x42524c00…") to its ASCII string,
// trimming trailing NUL padding (e.g. "BRL"). Returns "" on malformed input.
func decodeBytes32String(h string) string {
	h = strings.TrimPrefix(strings.TrimSpace(h), "0x")
	if h == "" {
		return ""
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(b), "\x00")
}
