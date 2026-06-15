// SPDX-License-Identifier: Apache-2.0

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

// paladinCBURL returns the Paladin HTTP API URL for the Central Bank node.
// Controlled by PALADIN_CB_URL env var; default is the spoke-a CB node.
func paladinCBURL() string {
	if u := os.Getenv("PALADIN_CB_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:8548"
}

type ptxSendRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type ptxSendResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Result  string    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type abiParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type abiEntry struct {
	Type   string     `json:"type"`
	Inputs []abiParam `json:"inputs"`
}

type paladinTx struct {
	Type     string      `json:"type"`
	Domain   string      `json:"domain,omitempty"`
	From     string      `json:"from"`
	To       interface{} `json:"to"`
	ABI      []abiEntry  `json:"abi,omitempty"`
	Function string      `json:"function"`
	Data     interface{} `json:"data"`
}

type zetoConstructorData struct {
	TokenName string `json:"tokenName"`
}

type ptxGetTxRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      int      `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

type txFullResult struct {
	Receipt *txReceipt `json:"receipt,omitempty"`
}

type txReceipt struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	ContractAddress string `json:"contractAddress"`
	Success         bool   `json:"success"`
	FailureMessage  string `json:"failureMessage,omitempty"`
}

type ptxGetTxResponse struct {
	Result *txFullResult `json:"result,omitempty"`
	Error  *rpcError     `json:"error,omitempty"`
}

// TestCreateZetoTokenInstance creates a Zeto_Anon token instance via
// the Paladin API. This is required before any mint/transfer/lock operations.
// The token contract address is written to .deployed-addrs.env as ZETO_TOKEN_ADDRESS.
//
// Prerequisites:
//   - Paladin nodes must be running (make paladin.start-spoke-a)
//   - ZetoFactory must be deployed (make paladin.deploy-zeto-spoke-a)
//
// Run:
//
//	SPOKE=spoke-a PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestCreateZetoTokenInstance -v -count=1
func TestCreateZetoTokenInstance(t *testing.T) {
	url := paladinCBURL()

	identity := fmt.Sprintf("funded_operator@%s-cb", spokeName())

	tx := paladinTx{
		Type:   "private",
		Domain: "zeto",
		From:   identity,
		To:     nil,
		ABI: []abiEntry{{
			Type:   "constructor",
			Inputs: []abiParam{{Name: "tokenName", Type: "string"}},
		}},
		Function: "",
		Data: zetoConstructorData{
			TokenName: "Zeto_Anon",
		},
	}

	reqBody := ptxSendRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ptx_sendTransaction",
		Params:  []interface{}{tx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	t.Logf("Sending Zeto token creation request to %s ...", url)
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
	txID := sendResp.Result
	t.Logf("Transaction submitted: %s", txID)

	// Poll for receipt
	var contractAddr string
	for i := 0; i < 60; i++ {
		time.Sleep(3 * time.Second)

		getTxBody, _ := json.Marshal(ptxGetTxRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "ptx_getTransactionFull",
			Params:  []string{txID},
		})
		getResp, err := http.Post(url, "application/json", bytes.NewReader(getTxBody))
		if err != nil {
			t.Logf("poll error: %v", err)
			continue
		}
		getRespBody, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()

		var txResp ptxGetTxResponse
		if err := json.Unmarshal(getRespBody, &txResp); err != nil {
			continue
		}
		if txResp.Result != nil && txResp.Result.Receipt != nil {
			if !txResp.Result.Receipt.Success {
				t.Fatalf("token creation transaction failed: %s", txResp.Result.Receipt.FailureMessage)
			}
			contractAddr = txResp.Result.Receipt.ContractAddress
			break
		}
	}

	if contractAddr == "" {
		t.Fatal("timeout waiting for Zeto token creation receipt")
	}

	fmt.Printf("\nZETO_TOKEN_ADDRESS=%s\n", contractAddr)
	t.Logf("Zeto tCeBM token instance: %s", contractAddr)
	writeOrUpdateEnvVar(t, addrsEnvFile(), "ZETO_TOKEN_ADDRESS", contractAddr)
}
