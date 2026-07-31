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
	"strings"
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
	if _, err := pollPenteTxReceipt(ctx, paladinURL, group.GenesisTransaction, nil, "group genesis"); err != nil {
		return "", fmt.Errorf("pente group genesis tx: %w", err)
	}
	return group.ID, nil
}

// resolveVerifier resolves a Paladin identity's verifier from paladinURL. For a
// REMOTE identity this drives the cross-node gRPC transport (the mTLS handshake to
// the peer node), so it doubles as a peer-readiness probe before pgroup_createGroup.
func resolveVerifier(ctx context.Context, paladinURL, identity string) (*penteRPCError, error) {
	var addr string
	return penteRPCCall(ctx, paladinURL, "ptx_resolveVerifier",
		[]interface{}{identity, "ecdsa:secp256k1", "eth_address"}, &addr)
}

// isTransientTransportErr reports whether a Paladin RPC error is a transient
// cross-node transport failure (the peer node is not connected YET) worth retrying,
// as opposed to a permanent fault. A node-identity mismatch (PD030011, e.g. a cert
// CN ≠ node name) is deterministic, so it is treated as PERMANENT and surfaced
// immediately instead of spinning until the deadline.
func isTransientTransportErr(e *penteRPCError) bool {
	if e == nil {
		return false
	}
	if strings.Contains(e.Message, "PD030011") {
		return false
	}
	for _, marker := range []string{
		"PD011206",         // TRANSPORT grpc returned error
		"PD030015",         // GRPC connection failed for endpoint
		"Unavailable",      // gRPC status: peer not accepting connections
		"connection error", // dial in progress
		"connection refused",
		"handshake", // TLS handshake mid-bring-up
		"EOF",
	} {
		if strings.Contains(e.Message, marker) {
			return true
		}
	}
	return false
}

// waitPentePeersReady blocks until every member identity resolves from paladinURL —
// i.e. the cross-node Paladin transport (mTLS) to each member's node is established.
// It is the peer-readiness gate for pgroup_createGroup: the create step runs in the
// join's soft tail and can fire before the bank's Paladin has connected to the CB's.
// Local members resolve instantly; remote members gate on the transport coming up.
// A permanent transport fault (e.g. a cert node-name mismatch) is surfaced at once.
func waitPentePeersReady(ctx context.Context, paladinURL string, members []string, interval time.Duration, log func(string)) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for _, member := range members {
		for attempt := 0; ; attempt++ {
			rpcErr, err := resolveVerifier(ctx, paladinURL, member)
			if err == nil && rpcErr == nil {
				break // resolved — transport to this member's node is up
			}
			if rpcErr != nil && !isTransientTransportErr(rpcErr) {
				return fmt.Errorf("resolve peer %s: %s", member, rpcErr.Message)
			}
			if log != nil && attempt%5 == 0 { // throttle progress (~every 5 polls)
				reason := "peer transport not ready"
				switch {
				case rpcErr != nil:
					reason = rpcErr.Message
				case err != nil:
					reason = err.Error()
				}
				log(fmt.Sprintf("waiting for Paladin peer %s: %s", member, reason))
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("peer %s not ready: %w", member, ctx.Err())
			case <-time.After(interval):
			}
		}
		if log != nil {
			log(fmt.Sprintf("Paladin peer %s ready", member))
		}
	}
	return nil
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
// returning the deployed contract address (empty if none). progress (nil-safe) is
// called with a heartbeat every ~5s while waiting so a long-running deploy step is
// visibly making progress instead of looking hung; label names what is being waited on.
func pollPenteTxReceipt(ctx context.Context, paladinURL, txID string, progress func(string), label string) (string, error) {
	// Poll until the receipt appears or ctx expires. The wait is bounded by the caller's step
	// timeout (PenteFXSetup, 20m) rather than a fixed iteration cap: a large contract (FXAgreement,
	// IR-compiled) can take well over a minute to confirm in a Pente group on a busy second
	// spoke, and a hard 60s cap produced a false "receipt timeout" that wedged deploy-fxa.
	start := time.Now()
	nextBeat := 5 * time.Second
	for {
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
		if progress != nil && time.Since(start) >= nextBeat {
			progress(fmt.Sprintf("waiting for %s tx receipt (%ds elapsed)", label, int(time.Since(start).Seconds())))
			nextBeat += 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("transaction %s receipt wait: %w", txID, ctx.Err())
		case <-time.After(1 * time.Second):
		}
	}
}

// fxaArtifact is the subset of the FXAgreement Foundry artifact we need.
type fxaArtifact struct {
	ABI      []json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// Participant roles in IdentityRegistryLibrary.ParticipantRole (enum order).
const (
	roleCentralBank    = 3 // CENTRAL_BANK — canGovern (proposeOnBehalf/settle)
	roleCommercialBank = 4 // COMMERCIAL_BANK — canTransact (propose/accept)
)

// zeroBytes32 is the ABI zero value for a bytes32 (e.g. an unset zkPointer).
const zeroBytes32 = "0x0000000000000000000000000000000000000000000000000000000000000000"

// deployContractInPente deploys a Foundry-compiled contract inside the privacy group via
// pgroup_sendTransaction (from = the deployer's node identity), passing ctorInput to the
// constructor, and returns the deployed in-group address.
func deployContractInPente(ctx context.Context, paladinURL, groupID, from, artifactPath string, ctorInput map[string]interface{}, progress func(string), label string) (string, error) {
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return "", fmt.Errorf("read artifact %s: %w", artifactPath, err)
	}
	var art fxaArtifact
	if err := json.Unmarshal(raw, &art); err != nil {
		return "", fmt.Errorf("parse artifact %s: %w", artifactPath, err)
	}
	if art.Bytecode.Object == "" {
		return "", fmt.Errorf("bytecode is empty for %s (run contracts.build)", artifactPath)
	}
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
		return "", fmt.Errorf("no constructor in artifact %s", artifactPath)
	}
	tx := map[string]interface{}{
		"domain":   penteDomain,
		"group":    groupID,
		"from":     from,
		"to":       nil,
		"bytecode": art.Bytecode.Object,
		"function": constructorABI,
		"input":    ctorInput,
	}
	if progress != nil {
		progress(fmt.Sprintf("deploying %s in group…", label))
	}
	var txID string
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_sendTransaction", []interface{}{tx}, &txID)
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", fmt.Errorf("pgroup_sendTransaction (deploy): %s", rpcErr.Message)
	}
	// Confirm the tx succeeded (base receipt), then read the deployed contract address from the
	// DOMAIN receipt — Pente private deploys do NOT expose contractAddress in the base receipt.
	if _, err := pollPenteTxReceipt(ctx, paladinURL, txID, progress, label); err != nil {
		return "", err
	}
	return penteDeployedAddress(ctx, paladinURL, txID, progress, label)
}

// penteDeployedAddress returns the in-group address of a contract deployed via
// pgroup_sendTransaction, read from the Pente domain receipt (ptx_getDomainReceipt). The base
// tx receipt does not carry contractAddress for private deploys, so this is polled until the
// domain receipt populates.
func penteDeployedAddress(ctx context.Context, paladinURL, txID string, progress func(string), label string) (string, error) {
	// Poll the domain receipt until the deployed address appears or ctx expires (bounded by the
	// caller's step timeout, not a fixed iteration cap — see pollPenteTxReceipt).
	start := time.Now()
	nextBeat := 5 * time.Second
	for {
		var res struct {
			Receipt struct {
				ContractAddress string `json:"contractAddress"`
			} `json:"receipt"`
		}
		rpcErr, err := penteRPCCall(ctx, paladinURL, "ptx_getDomainReceipt", []interface{}{penteDomain, txID}, &res)
		if err != nil {
			return "", err
		}
		if rpcErr == nil && res.Receipt.ContractAddress != "" {
			return res.Receipt.ContractAddress, nil
		}
		if progress != nil && time.Since(start) >= nextBeat {
			progress(fmt.Sprintf("waiting for %s address (%ds elapsed)", label, int(time.Since(start).Seconds())))
			nextBeat += 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("pente deploy %s: domain receipt wait: %w", txID, ctx.Err())
		case <-time.After(1 * time.Second):
		}
	}
}

// deployFXAInPente deploys FXAgreement inside the privacy group, wiring its constructor to the
// in-group IdentityRegistry address (which MUST have code in the group's private EVM — see
// setupBilateralFXAContext; a base-ledger address reverts canTransact with empty 0x).
func deployFXAInPente(ctx context.Context, paladinURL, groupID, from, artifactPath, identityRegistryAddr string, progress func(string)) (string, error) {
	return deployContractInPente(ctx, paladinURL, groupID, from, artifactPath,
		map[string]interface{}{"_identityRegistry": identityRegistryAddr}, progress, "FXAgreement")
}

// resolveAddress resolves a Paladin identity to its eth address within reach of paladinURL.
func resolveAddress(ctx context.Context, paladinURL, identity string) (string, error) {
	var addr string
	rpcErr, err := penteRPCCall(ctx, paladinURL, "ptx_resolveVerifier",
		[]interface{}{identity, "ecdsa:secp256k1", "eth_address"}, &addr)
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", fmt.Errorf("ptx_resolveVerifier %s: %s", identity, rpcErr.Message)
	}
	if addr == "" {
		return "", fmt.Errorf("ptx_resolveVerifier %s: empty address", identity)
	}
	return addr, nil
}

// registerParticipantInPente calls IdentityRegistry.registerParticipant inside the group,
// creating account in Pending with the given role. `from` MUST hold GOVERNANCE_ROLE (the
// registry admin / deployer). A follow-up verifyParticipantInPente is required to reach
// Verified — see setupBilateralFXAContext.
func registerParticipantInPente(ctx context.Context, paladinURL, groupID, from, registryAddr, account, name string, role int, progress func(string)) error {
	if progress != nil {
		progress(fmt.Sprintf("registering participant %s (role %d)…", name, role))
	}
	fn := map[string]interface{}{
		"type": "function", "name": "registerParticipant",
		"inputs": []map[string]interface{}{
			{"name": "account", "type": "address"},
			{"name": "name", "type": "string"},
			{"name": "role", "type": "uint8"},
			{"name": "zkPointer", "type": "bytes32"},
		},
	}
	tx := map[string]interface{}{
		"domain":   penteDomain,
		"group":    groupID,
		"from":     from,
		"to":       registryAddr,
		"function": fn,
		"input": map[string]interface{}{
			"account": account, "name": name,
			"role": fmt.Sprintf("%d", role), "zkPointer": zeroBytes32,
		},
	}
	var txID string
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_sendTransaction", []interface{}{tx}, &txID)
	if err != nil {
		return err
	}
	if rpcErr != nil {
		return fmt.Errorf("registerParticipant %s: %s", account, rpcErr.Message)
	}
	if _, err := pollPenteTxReceipt(ctx, paladinURL, txID, progress, "registerParticipant "+name); err != nil {
		return fmt.Errorf("registerParticipant %s receipt: %w", account, err)
	}
	return nil
}

// verifyParticipantInPente calls IdentityRegistry.verifyParticipant inside the group, promoting
// a previously registered account from Pending to Verified (step 2 of the two-step onboarding).
// `from` MUST hold VERIFIER_ROLE — the in-group registry grants it to the deployer/admin.
func verifyParticipantInPente(ctx context.Context, paladinURL, groupID, from, registryAddr, account, name string, progress func(string)) error {
	if progress != nil {
		progress(fmt.Sprintf("verifying participant %s…", name))
	}
	fn := map[string]interface{}{
		"type": "function", "name": "verifyParticipant",
		"inputs": []map[string]interface{}{
			{"name": "account", "type": "address"},
		},
	}
	tx := map[string]interface{}{
		"domain":   penteDomain,
		"group":    groupID,
		"from":     from,
		"to":       registryAddr,
		"function": fn,
		"input": map[string]interface{}{
			"account": account,
		},
	}
	var txID string
	rpcErr, err := penteRPCCall(ctx, paladinURL, "pgroup_sendTransaction", []interface{}{tx}, &txID)
	if err != nil {
		return err
	}
	if rpcErr != nil {
		return fmt.Errorf("verifyParticipant %s: %s", account, rpcErr.Message)
	}
	if _, err := pollPenteTxReceipt(ctx, paladinURL, txID, progress, "verifyParticipant "+name); err != nil {
		return fmt.Errorf("verifyParticipant %s receipt: %w", account, err)
	}
	return nil
}

// penteMember is a party to a bilateral FX context: its Paladin identity, legal name, and role.
type penteMember struct {
	Identity string
	Name     string
	Role     int
}

// setupBilateralFXAContext deploys a self-contained on-chain FX context inside an existing
// Pente group: (1) an IdentityRegistry with `deployer` as admin/governance, (2) each member
// registered then verified (two-step onboarding) with its role, (3) FXAgreement wired to that
// in-group registry. Returns
// the registry and FXAgreement in-group addresses. This is what makes on-chain propose/accept/
// settle actually succeed (the registry must live inside the group — see PLAN.md Phase 1a).
func setupBilateralFXAContext(ctx context.Context, paladinURL, groupID, deployer, idRegistryArtifact, fxaArtifact string, members []penteMember, progress func(string)) (registryAddr, fxaAddr string, err error) {
	emit := func(msg string) {
		if progress != nil {
			progress(msg)
		}
	}
	adminAddr, err := resolveAddress(ctx, paladinURL, deployer)
	if err != nil {
		return "", "", fmt.Errorf("resolve deployer %s: %w", deployer, err)
	}
	emit("deploying in-group IdentityRegistry")
	registryAddr, err = deployContractInPente(ctx, paladinURL, groupID, deployer, idRegistryArtifact,
		map[string]interface{}{"admin": adminAddr}, progress, "IdentityRegistry")
	if err != nil {
		return "", "", fmt.Errorf("deploy in-group IdentityRegistry: %w", err)
	}
	emit(fmt.Sprintf("IdentityRegistry deployed at %s; registering %d participant(s)", registryAddr, len(members)))
	// Resolve members to in-group addresses and dedupe: in local, entities share the dev
	// operator key, so multiple members can resolve to the SAME address. Registering it twice
	// (CENTRAL_BANK then COMMERCIAL_BANK) would leave it COMMERCIAL_BANK → canGovern() false.
	// Prefer the governing role on collision (CENTRAL_BANK also satisfies canTransact).
	type memberReg struct {
		name string
		role int
	}
	byAddr := map[string]*memberReg{}
	var order []string
	for _, m := range members {
		addr, err := resolveAddress(ctx, paladinURL, m.Identity)
		if err != nil {
			return "", "", fmt.Errorf("resolve member %s: %w", m.Identity, err)
		}
		if existing, ok := byAddr[addr]; ok {
			if m.Role == roleCentralBank {
				existing.role = roleCentralBank
				existing.name = m.Name
			}
			continue
		}
		byAddr[addr] = &memberReg{name: m.Name, role: m.Role}
		order = append(order, addr)
	}
	for _, addr := range order {
		r := byAddr[addr]
		// Two-step onboarding: register (Pending) then verify (Pending -> Verified) so the member
		// passes canTransact/canGovern in the in-group FXAgreement. The deployer holds both
		// GOVERNANCE_ROLE and VERIFIER_ROLE (in-group registry constructor grants both to admin).
		if err := registerParticipantInPente(ctx, paladinURL, groupID, deployer, registryAddr, addr, r.name, r.role, progress); err != nil {
			return "", "", err
		}
		if err := verifyParticipantInPente(ctx, paladinURL, groupID, deployer, registryAddr, addr, r.name, progress); err != nil {
			return "", "", err
		}
	}
	emit("deploying FXAgreement wired to the in-group registry")
	fxaAddr, err = deployFXAInPente(ctx, paladinURL, groupID, deployer, fxaArtifact, registryAddr, progress)
	if err != nil {
		return "", "", fmt.Errorf("deploy FXAgreement: %w", err)
	}
	return registryAddr, fxaAddr, nil
}
