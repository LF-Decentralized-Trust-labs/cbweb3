// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// pente.go implements the native Paladin Pente client used by mode:join (US2/US3)
// to create a bilateral privacy group (CB↔bank) and deploy FXAgreement inside it.
// It speaks Paladin's pgroup_* / ptx_* JSON-RPC directly — parametrized by the two
// participants' node identities, never a hardcoded bank list (replaces the
// reference create_pente_context / deploy_fxagreement_pente scripts).

const penteDomain = "pente"

// paladinIdentity is the signing identity reference for a Paladin node: the
// funded operator key scoped to the on-chain-registered node name.
func paladinIdentity(nodeName string) string { return "funded_operator@" + nodeName }

type penteRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func penteRPCCall(ctx context.Context, url, method string, params interface{}, out interface{}) (*penteRPCError, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("new request %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", method, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *penteRPCError  `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s response: %w (body: %s)", method, err, string(data))
	}
	if envelope.Error != nil {
		return envelope.Error, nil
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return nil, fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil, nil
}

type pentePrivacyGroup struct {
	ID                 string  `json:"id"`
	GenesisTransaction string  `json:"genesisTransaction"`
	ContractAddress    *string `json:"contractAddress"`
}

type penteTxReceipt struct {
	ContractAddress string `json:"contractAddress"`
	Success         bool   `json:"success"`
	FailureMessage  string `json:"failureMessage,omitempty"`
}

// createPenteGroup creates a bilateral Pente privacy group whose members are the
// given node identities, and waits for its genesis transaction to confirm.
// Returns the group id (hex bytes32).
func createPenteGroup(ctx context.Context, paladinURL, name string, members []string) (string, error) {
	input := map[string]interface{}{
		"domain":  penteDomain,
		"members": members,
		"name":    name,
		"configuration": map[string]interface{}{
			"endorsementType": "group_scoped_identities",
			"evmVersion":      "shanghai",
		},
	}
	var group pentePrivacyGroup
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_createGroup", []interface{}{input}, &group)
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", fmt.Errorf("pgroup_createGroup: %s", rpcErr.Message)
	}
	if group.ID == "" {
		return "", fmt.Errorf("pgroup_createGroup returned empty group id")
	}
	// The group is created asynchronously; wait for the genesis tx to confirm.
	if _, err := pollPenteTxReceipt(ctx, paladinURL, group.GenesisTransaction); err != nil {
		return "", fmt.Errorf("pente group genesis tx: %w", err)
	}
	return group.ID, nil
}

// penteGroupExists returns true if a group with the given id is resolvable.
func penteGroupExists(ctx context.Context, paladinURL, groupID string) bool {
	var group pentePrivacyGroup
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_getGroupById", []string{penteDomain, groupID}, &group)
	return err == nil && rpcErr == nil && group.ID != ""
}

// waitPenteGroupReady polls pgroup_getGroupById until the group's contractAddress
// is populated (the group is ready to receive transactions).
func waitPenteGroupReady(ctx context.Context, paladinURL, groupID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var group pentePrivacyGroup
		rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_getGroupById", []string{penteDomain, groupID}, &group)
		if err == nil && rpcErr == nil && group.ContractAddress != nil && *group.ContractAddress != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return fmt.Errorf("pente group %s not ready within %s", groupID, timeout)
}

// pollPenteTxReceipt polls ptx_getTransactionFull until the receipt is available,
// returning the deployed contract address (empty if none).
func pollPenteTxReceipt(ctx context.Context, paladinURL, txID string) (string, error) {
	for i := 0; i < 60; i++ {
		var res struct {
			Receipt *penteTxReceipt `json:"receipt"`
		}
		rpcErr, err := penteRPCCall(ctx, paladinURL, "ptx_getTransactionFull", []string{txID}, &res)
		if err != nil {
			return "", err
		}
		if rpcErr == nil && res.Receipt != nil {
			if !res.Receipt.Success {
				return "", fmt.Errorf("transaction failed: %s", res.Receipt.FailureMessage)
			}
			return res.Receipt.ContractAddress, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return "", fmt.Errorf("transaction %s receipt timeout", txID)
}

// fxaArtifact is the subset of the FXAgreement Foundry artifact we need.
type fxaArtifact struct {
	ABI      []json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// deployFXAInPente deploys FXAgreement inside the privacy group via
// pgroup_sendTransaction (from = the calling node's identity), passing the
// participant-registry address to the constructor, and returns its address.
func deployFXAInPente(ctx context.Context, paladinURL, groupID, from, artifactPath, identityRegistryAddr string) (string, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return "", fmt.Errorf("read FXAgreement artifact %s: %w", artifactPath, err)
	}
	var art fxaArtifact
	if err := json.Unmarshal(raw, &art); err != nil {
		return "", fmt.Errorf("parse FXAgreement artifact: %w", err)
	}
	if art.Bytecode.Object == "" {
		return "", fmt.Errorf("FXAgreement bytecode is empty (run contracts.build)")
	}
	// Find the constructor ABI entry.
	var constructorABI json.RawMessage
	for _, e := range art.ABI {
		var probe struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(e, &probe) == nil && probe.Type == "constructor" {
			constructorABI = e
			break
		}
	}
	if constructorABI == nil {
		return "", fmt.Errorf("no constructor in FXAgreement ABI")
	}

	tx := map[string]interface{}{
		"domain":   penteDomain,
		"group":    groupID,
		"from":     from,
		"to":       nil,
		"bytecode": art.Bytecode.Object,
		"function": constructorABI,
		"input":    map[string]interface{}{"_identityRegistry": identityRegistryAddr},
	}
	var txID string
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_sendTransaction", []interface{}{tx}, &txID)
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", fmt.Errorf("pgroup_sendTransaction: %s", rpcErr.Message)
	}
	addr, err := pollPenteTxReceipt(ctx, paladinURL, txID)
	if err != nil {
		return "", err
	}
	return addr, nil
}
