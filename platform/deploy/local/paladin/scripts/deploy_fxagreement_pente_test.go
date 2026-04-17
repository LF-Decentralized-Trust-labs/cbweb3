package scripts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type fxAgreementConstructorData struct {
	IdentityRegistry string `json:"_identityRegistry"`
}

type forgeArtifact struct {
	ABI      []interface{} `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// penteEVMTxInput maps to PrivacyGroupEVMTXInput for pgroup_sendTransaction.
type penteEVMTxInput struct {
	Domain   string      `json:"domain"`
	Group    string      `json:"group"` // hex bytes32 group ID
	From     string      `json:"from"`
	To       interface{} `json:"to"` // null for deploy
	Bytecode string      `json:"bytecode,omitempty"`
	Function interface{} `json:"function,omitempty"` // ABI entry (single object)
	Input    interface{} `json:"input,omitempty"`    // constructor/function args
}

// penteEVMCallInput maps to PrivacyGroupEVMCall for pgroup_call (read-only).
type penteEVMCallInput struct {
	Domain   string      `json:"domain"`
	Group    string      `json:"group"`
	From     string      `json:"from"`
	To       string      `json:"to"`
	Function interface{} `json:"function,omitempty"`
	Input    interface{} `json:"input,omitempty"`
}

type pGroupCallResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// TestDeployFXAgreementPente deploys the FXAgreement contract within
// the previously created bilateral Pente context using pgroup_sendTransaction.
//
// Prerequisites:
//   - Paladin nodes must be running (make paladin.start-spoke-a)
//   - IdentityRegistry must be deployed (make paladin.deploy-registry-spoke-a)
//   - Bilateral Pente context must exist (make paladin.create-pente-context-spoke-a)
//   - FXAgreement must be compiled (make contracts.build)
//   - REGISTRY_CONTRACT_ADDRESS env var must be set
//   - PENTE_CONTEXT_GROUP_ID env var must be set (hex bytes32 from pgroup_createGroup)
//
// Run:
//
//	SPOKE=spoke-a REGISTRY_CONTRACT_ADDRESS=0x... PENTE_CONTEXT_GROUP_ID=0x... PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestDeployFXAgreementPente -v -count=1
func TestDeployFXAgreementPente(t *testing.T) {
	registryAddr := os.Getenv("REGISTRY_CONTRACT_ADDRESS")
	if registryAddr == "" {
		t.Fatalf("REGISTRY_CONTRACT_ADDRESS not set")
	}

	penteGroupID := os.Getenv("PENTE_CONTEXT_GROUP_ID")
	if penteGroupID == "" {
		t.Fatalf("PENTE_CONTEXT_GROUP_ID not set — run paladin.create-pente-context first")
	}

	paladinURL := paladinCBURL()
	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	if len(registryAddr) == 40 {
		registryAddr = "0x" + registryAddr
	}

	// Load FXAgreement bytecode and ABI from Forge artifact.
	// Tests run from deploy/local/paladin/scripts, while contract artifacts are in repo/contracts/out.
	artifactPath := "../../../../contracts/out/FXAgreement.sol/FXAgreement.json"
	t.Logf("Loading FXAgreement artifact from %s...", artifactPath)

	artifact, err := loadForgeArtifact(artifactPath)
	if err != nil {
		t.Fatalf("failed to load FXAgreement artifact: %v", err)
	}

	if artifact.Bytecode.Object == "" || artifact.Bytecode.Object == "0x" {
		t.Fatalf("FXAgreement bytecode is empty — ensure contracts are compiled with `make contracts.build`")
	}
	if len(artifact.ABI) == 0 {
		t.Fatalf("FXAgreement ABI is empty in artifact")
	}

	t.Logf("Loaded FXAgreement: ABI entries=%d, bytecode length=%d", len(artifact.ABI), len(artifact.Bytecode.Object))

	// Group creation is asynchronous; wait for the genesis transaction confirmation
	// so pgroup_sendTransaction does not fail with "Privacy group is not ready".
	if err := waitForPenteGroupReady(paladinURL, penteGroupID); err != nil {
		t.Fatalf("pente group not ready: %v", err)
	}

	// Find constructor ABI entry
	var constructorABI interface{}
	for _, entry := range artifact.ABI {
		if m, ok := entry.(map[string]interface{}); ok {
			if m["type"] == "constructor" {
				constructorABI = entry
				break
			}
		}
	}
	if constructorABI == nil {
		t.Fatalf("no constructor found in FXAgreement ABI")
	}

	// Deploy FXAgreement into existing Pente group via pgroup_sendTransaction
	deployTx := penteEVMTxInput{
		Domain:   "pente",
		Group:    penteGroupID,
		From:     identity,
		To:       nil,
		Bytecode: artifact.Bytecode.Object,
		Function: constructorABI,
		Input: map[string]interface{}{
			"_identityRegistry": registryAddr,
		},
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgroup_sendTransaction",
		Params:  []interface{}{deployTx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Deploying FXAgreement contract in Pente group %s...", penteGroupID)
	t.Logf("  Registry: %s", registryAddr)
	t.Logf("  Pente URL: %s", paladinURL)

	resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", paladinURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var sendResp ptxSendResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if sendResp.Error != nil {
		t.Fatalf("RPC error: %s", sendResp.Error.Message)
	}

	txId := sendResp.Result
	t.Logf("FXAgreement deployment transaction submitted: %s", txId)

	t.Logf("Polling for FXAgreement deployment confirmation...")
	contractAddr, err := pollTxReceipt(paladinURL, txId)
	if err != nil {
		t.Fatalf("FXAgreement deployment failed: %v", err)
	}

	t.Logf("✓ FXAgreement contract deployed successfully in Pente group")
	t.Logf("  Contract address: %s", contractAddr)

	if contractAddr != "" {
		writeOrUpdateEnvVar(t, addrsEnvFile(), "FX_AGREEMENT_ADDRESS", contractAddr)
	} else {
		t.Logf("FXAgreement contractAddress not present in receipt; storing deployment tx id for verification")
	}
	writeOrUpdateEnvVar(t, addrsEnvFile(), "FX_AGREEMENT_DEPLOYED_AT", txId)
}

// waitForPenteGroupReady waits for the group genesis transaction to be confirmed on-chain.
func waitForPenteGroupReady(paladinURL, groupID string) error {
	// Fetch group metadata to obtain genesisTransaction UUID.
	getReq := pGroupGetByIdRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgroup_getGroupById",
		Params:  []string{"pente", groupID},
	}

	body, err := json.Marshal(getReq)
	if err != nil {
		return fmt.Errorf("marshal pgroup_getGroupById: %w", err)
	}

	resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pgroup_getGroupById POST: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pgroup_getGroupById status %d: %s", resp.StatusCode, string(respBody))
	}

	var groupResp pGroupGetByIdResponse
	if err := json.Unmarshal(respBody, &groupResp); err != nil {
		return fmt.Errorf("unmarshal pgroup_getGroupById: %w", err)
	}
	if groupResp.Error != nil {
		return fmt.Errorf("pgroup_getGroupById RPC error: %s", groupResp.Error.Message)
	}
	if groupResp.Result == nil {
		return fmt.Errorf("pgroup_getGroupById returned empty result")
	}
	if groupResp.Result.GenesisTransaction == "" {
		return fmt.Errorf("group %s has empty genesisTransaction", groupID)
	}

	genesisTxID := groupResp.Result.GenesisTransaction
	if _, err := pollTxReceipt(paladinURL, genesisTxID); err != nil {
		return fmt.Errorf("group %s genesis transaction %s not confirmed: %w", groupID, genesisTxID, err)
	}
	return nil
}

// TestVerifyFXAgreementPenteDeploy verifies that the FXAgreement contract
// is deployed and callable within the Pente context via pgroup_call.
//
// Prerequisites:
//   - FXAgreement must be deployed (TestDeployFXAgreementPente)
//   - FX_AGREEMENT_ADDRESS env var must be set
//   - PENTE_CONTEXT_GROUP_ID env var must be set
//
// Run:
//
//	SPOKE=spoke-a FX_AGREEMENT_ADDRESS=0x... PENTE_CONTEXT_GROUP_ID=0x... PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestVerifyFXAgreementPenteDeploy -v -count=1
func TestVerifyFXAgreementPenteDeploy(t *testing.T) {
	contractAddr := os.Getenv("FX_AGREEMENT_ADDRESS")
	txID := os.Getenv("FX_AGREEMENT_DEPLOYED_AT")
	if txID == "" {
		t.Fatalf("FX_AGREEMENT_DEPLOYED_AT not set")
	}

	penteGroupID := os.Getenv("PENTE_CONTEXT_GROUP_ID")
	if penteGroupID == "" {
		t.Fatalf("PENTE_CONTEXT_GROUP_ID not set — run paladin.create-pente-context first")
	}

	if len(contractAddr) == 40 {
		contractAddr = "0x" + contractAddr
	}

	paladinURL := paladinCBURL()
	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	// First verify the deploy transaction itself has a successful receipt.
	if _, err := pollTxReceipt(paladinURL, txID); err != nil {
		t.Fatalf("FXAgreement deploy tx %s not confirmed: %v", txID, err)
	}

	if contractAddr == "" {
		t.Logf("FX_AGREEMENT_ADDRESS not available; deployment tx %s confirmed successfully", txID)
		return
	}

	dummyTradeId := "0x0000000000000000000000000000000000000000000000000000000000000000"

	// Use pgroup_call for read-only view function
	callInput := penteEVMCallInput{
		Domain: "pente",
		Group:  penteGroupID,
		From:   identity,
		To:     contractAddr,
		Function: map[string]interface{}{
			"type":    "function",
			"name":    "getAgreement",
			"inputs":  []map[string]interface{}{{"name": "tradeId", "type": "bytes32"}},
			"outputs": []map[string]interface{}{{"name": "", "type": "tuple", "internalType": "struct FXAgreementLibrary.FxAgreement"}},
		},
		Input: map[string]interface{}{
			"tradeId": dummyTradeId,
		},
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgroup_call",
		Params:  []interface{}{callInput},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Verifying FXAgreement contract is callable via pgroup_call...")

	resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", paladinURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verification failed: %s", string(respBody))
	}

	var callResp pGroupCallResponse
	if err := json.Unmarshal(respBody, &callResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("contract call error: %s", callResp.Error.Message)
	}

	t.Logf("FXAgreement contract verification PASSED")
	t.Logf("Contract is deployed and callable in Pente group %s", penteGroupID)
}

// loadForgeArtifact reads the compiled contract artifact from Forge output
func loadForgeArtifact(path string) (*forgeArtifact, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", absPath, err)
	}

	var artifact forgeArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("unmarshal artifact: %w", err)
	}

	return &artifact, nil
}
