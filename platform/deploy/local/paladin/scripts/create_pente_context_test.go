package scripts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestCreatePenteContextBilateral creates a bilateral private context in Pente
// for a specific participant pair (groupId = hash of parties involved).
// This context is required before deploying FXAgreement on Pente.
//
// Prerequisites:
//   - Paladin nodes must be running (make paladin.start-spoke-a)
//   - IdentityRegistry must be deployed
//
// Run:
//
//	SPOKE=spoke-a PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestCreatePenteContextBilateral -v -count=1
func TestCreatePenteContextBilateral(t *testing.T) {
	url := paladinCBURL()
	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	// groupId is a unique identifier for the bilateral context
	// In production, this would be derived from participant identities
	groupId := fmt.Sprintf("fx-agreement-bilateral-%s", time.Now().Format("20060102150405"))

	// Pente context creation via ptx_sendTransaction
	// Type="private" with Domain="pente" to create a private bilateral context
	contextTx := paladinTx{
		Type:   "private",
		Domain: "pente",
		From:   identity,
		To:     nil,
		ABI: []abiEntry{{
			Type: "constructor",
			Inputs: []abiParam{
				{Name: "groupId", Type: "string"},
				{Name: "domain", Type: "string"},
				{Name: "description", Type: "string"},
			},
		}},
		Function: "",
		Data: map[string]interface{}{
			"groupId":     groupId,
			"domain":      "fx-agreement",
			"description": "Bilateral FX Agreement private context (Zeto + Pente local operation)",
		},
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ptx_sendTransaction",
		Params:  []interface{}{contextTx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Creating Pente bilateral context (%s) on %s ...", groupId, url)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
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
	t.Logf("Pente context creation transaction submitted: %s", txId)

	// Poll for transaction status
	t.Logf("Polling for Pente context creation transaction confirmation...")
	contextAddress, err := pollTxReceipt(url, txId)
	if err != nil {
		t.Fatalf("context creation failed: %v", err)
	}

	t.Logf("Pente bilateral context created successfully")
	t.Logf("GroupID: %s", groupId)
	t.Logf("Context address: %s", contextAddress)

	// Write to .deployed-addrs.env
	if err := appendEnvFile(".deployed-addrs.env", []string{
		fmt.Sprintf("PENTE_CONTEXT_GROUP_ID=%s", groupId),
		fmt.Sprintf("PENTE_CONTEXT_ADDRESS=%s", contextAddress),
	}); err != nil {
		t.Logf("warning: failed to write env: %v", err)
	}
}

// spokeName returns the SPOKE env var or defaults to spoke-a
func spokeName() string {
	if s := os.Getenv("SPOKE"); s != "" {
		return s
	}
	return "spoke-a"
}

// pollTxReceipt polls ptx_getTransaction until receipt is available
func pollTxReceipt(paladinURL, txId string) (string, error) {
	maxAttempts := 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		reqBody := ptxGetTxRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "ptx_getTransaction",
			Params:  []string{txId},
		}

		body, _ := json.Marshal(reqBody)
		resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("ptx_getTransaction: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var txResp ptxGetTxResponse
		if err := json.Unmarshal(respBody, &txResp); err != nil {
			return "", fmt.Errorf("unmarshal: %w", err)
		}

		if txResp.Result != nil && txResp.Result.Receipt != nil {
			receipt := txResp.Result.Receipt
			if !receipt.Success {
				return "", fmt.Errorf("transaction failed: %s", receipt.FailureMessage)
			}
			return receipt.ContractAddress, nil
		}

		time.Sleep(1 * time.Second)
	}

	return "", fmt.Errorf("transaction timeout after %d attempts", maxAttempts)
}

// appendEnvFile appends key=value pairs to an env file
func appendEnvFile(path string, lines []string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range lines {
		if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
			return err
		}
	}
	return nil
}
