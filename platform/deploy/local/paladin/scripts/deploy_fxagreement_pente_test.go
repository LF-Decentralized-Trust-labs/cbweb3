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
	IdentityRegistry string `json:"identityRegistry"`
}

type forgeArtifact struct {
	ABI      []interface{} `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// TestDeployFXAgreementPente deploys the FXAgreement contract within
// the previously created bilateral Pente context. This contract manages
// the lifecycle of FX agreements and requires the IdentityRegistry address.
//
// Prerequisites:
//   - Paladin nodes must be running (make paladin.start-spoke-a)
//   - IdentityRegistry must be deployed (make paladin.deploy-registry-spoke-a)
//   - Bilateral Pente context must exist (make paladin.create-pente-context-spoke-a)
//   - FXAgreement must be compiled (make contracts.build)
//   - REGISTRY_CONTRACT_ADDRESS env var must be set
//
// Run:
//
//	SPOKE=spoke-a REGISTRY_CONTRACT_ADDRESS=0x... PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestDeployFXAgreementPente -v -count=1
func TestDeployFXAgreementPente(t *testing.T) {
	registryAddr := os.Getenv("REGISTRY_CONTRACT_ADDRESS")
	if registryAddr == "" {
		t.Fatalf("REGISTRY_CONTRACT_ADDRESS not set")
	}

	paladinURL := paladinCBURL()
	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	// Ensure address is properly formatted
	if len(registryAddr) == 42 && registryAddr[:2] == "0x" {
		// Already formatted
	} else if len(registryAddr) == 40 {
		registryAddr = "0x" + registryAddr
	}

	// Load FXAgreement bytecode and ABI from Forge artifact
	artifactPath := "../../../contracts/out/FXAgreement.sol/FXAgreement.json"
	t.Logf("Loading FXAgreement artifact from %s...", artifactPath)

	artifact, err := loadForgeArtifact(artifactPath)
	if err != nil {
		t.Fatalf("failed to load FXAgreement artifact: %v", err)
	}

	// Ensure bytecode is not empty
	if artifact.Bytecode.Object == "" || artifact.Bytecode.Object == "0x" {
		t.Fatalf("FXAgreement bytecode is empty — ensure contracts are compiled with `make contracts.build`")
	}

	if len(artifact.ABI) == 0 {
		t.Fatalf("FXAgreement ABI is empty in artifact")
	}

	t.Logf("Loaded FXAgreement: ABI entries=%d, bytecode length=%d", len(artifact.ABI), len(artifact.Bytecode.Object))

	// Deploy FXAgreement in Pente private context
	// Type="private" with Domain="pente" to deploy within the private context
	deployTxMap := map[string]interface{}{
		"type":   "private",
		"domain": "pente",
		"from":   identity,
		"to":     nil,
		"data": map[string]interface{}{
			"identityRegistry": registryAddr,
		},
		"bytecode": artifact.Bytecode.Object,
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ptx_sendTransaction",
		Params:  []interface{}{deployTxMap},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Deploying FXAgreement contract in Pente context...")
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

	// Poll for transaction status
	t.Logf("Polling for FXAgreement deployment confirmation...")
	contractAddr, err := pollTxReceipt(paladinURL, txId)
	if err != nil {
		t.Fatalf("FXAgreement deployment failed: %v", err)
	}

	t.Logf("✓ FXAgreement contract deployed successfully in Pente context")
	t.Logf("  Contract address: %s", contractAddr)

	// Write to .deployed-addrs.env
	if err := appendEnvFile(".deployed-addrs.env", []string{
		fmt.Sprintf("FX_AGREEMENT_ADDRESS=%s", contractAddr),
		fmt.Sprintf("FX_AGREEMENT_DEPLOYED_AT=%s", txId),
	}); err != nil {
		t.Logf("warning: failed to write env: %v", err)
	}
}

// TestVerifyFXAgreementPenteDeploy verifies that the FXAgreement contract
// is deployed and callable within the Pente context.
//
// Prerequisites:
//   - FXAgreement must be deployed (TestDeployFXAgreementPente)
//   - FX_AGREEMENT_ADDRESS env var must be set
//
// Run:
//
//	SPOKE=spoke-a FX_AGREEMENT_ADDRESS=0x... PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestVerifyFXAgreementPenteDeploy -v -count=1
func TestVerifyFXAgreementPenteDeploy(t *testing.T) {
	contractAddr := os.Getenv("FX_AGREEMENT_ADDRESS")
	if contractAddr == "" {
		t.Fatalf("FX_AGREEMENT_ADDRESS not set")
	}

	if len(contractAddr) == 40 {
		contractAddr = "0x" + contractAddr
	}

	paladinURL := paladinCBURL()
	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	// Call getAgreement with a dummy tradeId to verify contract is callable
	dummyTradeId := "0x0000000000000000000000000000000000000000000000000000000000000000"

	callTx := paladinTx{
		Type:     "private",
		Domain:   "pente",
		From:     identity,
		To:       contractAddr,
		Function: "getAgreement",
		Data: map[string]interface{}{
			"tradeId": dummyTradeId,
		},
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ptx_sendTransaction",
		Params:  []interface{}{callTx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Verifying FXAgreement contract is callable...")

	resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", paladinURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verification failed: %s", string(respBody))
	}

	var callResp ptxSendResponse
	if err := json.Unmarshal(respBody, &callResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("contract call error: %s", callResp.Error.Message)
	}

	t.Logf("FXAgreement contract verification PASSED")
	t.Logf("Contract is deployed and callable in Pente context")
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
