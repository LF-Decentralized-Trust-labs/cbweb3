// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// selectorHex returns the 4-byte function selector for an ABI signature, hex-encoded.
func selectorHex(sig string) string {
	return hex.EncodeToString(crypto.Keccak256([]byte(sig))[:4])
}

// HubCBRegistered reports whether the CB is BOTH a verified participant
// (isWhitelisted) AND a liquidity provider (isLiquidityProvider) on the hub
// IdentityRegistry. It is register-cb's idempotency Check (FR-002/SC-002): true
// ⇒ skip the step on re-apply. The spec names the participant check
// "isParticipant"; the contract exposes it as isWhitelisted(address) (KYC
// status == Verified).
func HubCBRegistered(ctx context.Context, rpcURL, identityRegistry, cbAddress string) (bool, error) {
	whitelisted, err := ethCallBool(ctx, rpcURL, identityRegistry, "isWhitelisted(address)", cbAddress)
	if err != nil {
		return false, err
	}
	if !whitelisted {
		return false, nil
	}
	return ethCallBool(ctx, rpcURL, identityRegistry, "isLiquidityProvider(address)", cbAddress)
}

// contractHasCode reports whether addr has non-empty bytecode on-chain
// (eth_getCode != "0x"). It makes the deploy Checks self-healing: a broadcast
// file alone does not prove the contract exists on the CURRENT chain — a
// recreated volume or reset chain leaves the broadcast stale, so the deploy must
// re-run. A transient RPC error is surfaced to the caller, which treats it as
// "not deployed" (safe: re-run rather than skip).
func contractHasCode(ctx context.Context, rpcURL, addr string) (bool, error) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "eth_getCode",
		"params": []any{addr, "latest"},
	})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	if out.Error != nil {
		return false, fmt.Errorf("eth_getCode %s: %s", addr, out.Error.Message)
	}
	return strings.TrimPrefix(out.Result, "0x") != "", nil
}

// ethCallBool performs an eth_call to a `foo(address) view returns (bool)`
// method and reports the decoded boolean.
func ethCallBool(ctx context.Context, rpcURL, to, sig, addr string) (bool, error) {
	selector := crypto.Keccak256([]byte(sig))[:4]
	arg := make([]byte, 32)
	copy(arg[12:], common.HexToAddress(addr).Bytes()) // left-pad the 20-byte address to 32
	data := "0x" + hex.EncodeToString(append(selector, arg...))

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "eth_call",
		"params": []any{map[string]string{"to": to, "data": data}, "latest"},
	})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	if out.Error != nil {
		return false, fmt.Errorf("eth_call %s: %s", sig, out.Error.Message)
	}
	// A bool return is a 32-byte word; any non-zero value ⇒ true.
	return strings.Trim(strings.TrimPrefix(out.Result, "0x"), "0") != "", nil
}
