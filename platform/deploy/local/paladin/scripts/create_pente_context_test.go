package scripts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// pentePrivacyGroupInput is the input payload for pgroup_createGroup.
type pentePrivacyGroupInput struct {
	Domain        string                 `json:"domain"`
	Members       []string               `json:"members"`
	Name          string                 `json:"name,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// pentePrivacyGroup is the result returned by pgroup_createGroup.
type pentePrivacyGroup struct {
	ID                 string   `json:"id"`
	Domain             string   `json:"domain"`
	Name               string   `json:"name"`
	Members            []string `json:"members"`
	GenesisTransaction string   `json:"genesisTransaction"`
	ContractAddress    *string  `json:"contractAddress"`
}

// pGroupCreateRequest is the JSON-RPC request for pgroup_createGroup.
type pGroupCreateRequest struct {
	JSONRPC string                   `json:"jsonrpc"`
	ID      int                      `json:"id"`
	Method  string                   `json:"method"`
	Params  []pentePrivacyGroupInput `json:"params"`
}

// pGroupCreateResponse is the JSON-RPC response for pgroup_createGroup.
type pGroupCreateResponse struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  *pentePrivacyGroup `json:"result,omitempty"`
	Error   *rpcError          `json:"error,omitempty"`
}

// pGroupGetByIdRequest is the JSON-RPC request for pgroup_getGroupById.
type pGroupGetByIdRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      int      `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

// pGroupGetByIdResponse is the JSON-RPC response for pgroup_getGroupById.
type pGroupGetByIdResponse struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  *pentePrivacyGroup `json:"result,omitempty"`
	Error   *rpcError          `json:"error,omitempty"`
}

// pollPenteGroupContractAddress polls pgroup_getGroupById and returns contractAddress when available.
// A group can be created successfully with contractAddress still null for some time, so this helper
// returns empty string without error when the timeout is reached.
func pollPenteGroupContractAddress(paladinURL, domain, groupIDHex string) (string, error) {
	maxAttempts := 15 // best-effort, non-blocking for pipeline
	for attempt := 0; attempt < maxAttempts; attempt++ {
		reqBody := pGroupGetByIdRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "pgroup_getGroupById",
			Params:  []string{domain, groupIDHex},
		}

		body, _ := json.Marshal(reqBody)
		resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("pgroup_getGroupById: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var getResp pGroupGetByIdResponse
		if err := json.Unmarshal(respBody, &getResp); err != nil {
			return "", fmt.Errorf("unmarshal: %w", err)
		}

		if getResp.Error != nil {
			return "", fmt.Errorf("pgroup_getGroupById error: %s", getResp.Error.Message)
		}

		if getResp.Result != nil && getResp.Result.ContractAddress != nil && *getResp.Result.ContractAddress != "" {
			return *getResp.Result.ContractAddress, nil
		}

		time.Sleep(1 * time.Second)
	}

	return "", nil
}

// TestCreatePenteContextBilateral creates a bilateral private context in Pente
// using pgroup_createGroup with endorsementType=group_scoped_identities.
// This context is required before deploying FXAgreement on Pente.
//
// Prerequisites:
//   - Paladin nodes must be running (make paladin.start-spoke-a)
//
// Run:
//
//	SPOKE=spoke-a PALADIN_CB_URL=http://127.0.0.1:31648 go test ./scripts/ -run TestCreatePenteContextBilateral -v -count=1
func TestCreatePenteContextBilateral(t *testing.T) {
	url := paladinCBURL()
	spoke := spokeName()
	members := bilateralMembers(spoke)
	cbOnly := []string{fmt.Sprintf("funded_operator@%s-cb", spoke)}

	groupName := fmt.Sprintf("fx-agreement-bilateral-%s", time.Now().Format("20060102150405"))
	usedMembers := members

	t.Logf("Creating Pente bilateral context (%s) on %s ...", groupName, url)
	createResp, err := createPenteGroup(url, groupName, members)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
			t.Logf("Bilateral creation timed out resolving counterparties; retrying with CB-only member set: %v", cbOnly)
			createResp, err = createPenteGroup(url, groupName+"-cb", cbOnly)
			usedMembers = cbOnly
		}
		if err != nil {
			t.Fatalf("failed to create Pente context: %v", err)
		}
	}

	group := createResp.Result
	groupID := group.ID
	genesisTxID := group.GenesisTransaction

	t.Logf("Pente group submitted: id=%s genesisTransaction=%s", groupID, genesisTxID)
	t.Logf("Group members: %v", usedMembers)

	// A successful pgroup_createGroup response still requires on-chain genesis confirmation.
	if _, err := pollTxReceipt(url, genesisTxID); err != nil {
		t.Fatalf("Pente group genesis transaction failed: %v", err)
	}

	// contractAddress may be null immediately after creation; keep this as best-effort.
	t.Logf("Polling for Pente group contractAddress (best-effort)...")
	contextAddress, err := pollPenteGroupContractAddress(url, "pente", groupID)
	if err != nil {
		t.Fatalf("Pente group polling failed: %v", err)
	}

	t.Logf("Pente bilateral context created successfully")
	t.Logf("GroupID: %s", groupID)
	t.Logf("GroupName: %s", groupName)
	if contextAddress != "" {
		t.Logf("Context address: %s", contextAddress)
	} else {
		t.Logf("Context address is not available yet (contractAddress=null); continuing with GroupID")
	}

	writeOrUpdateEnvVar(t, addrsEnvFile(), "PENTE_CONTEXT_GROUP_ID", groupID)
	if contextAddress != "" {
		writeOrUpdateEnvVar(t, addrsEnvFile(), "PENTE_CONTEXT_ADDRESS", contextAddress)
	}
}

// createPenteGroup invokes pgroup_createGroup and validates the response.
func createPenteGroup(url, groupName string, members []string) (*pGroupCreateResponse, error) {
	// pgroup_createGroup is the correct Paladin v0.15 API for creating a Pente
	// privacy group. It requires endorsementType="group_scoped_identities".
	reqBody := pGroupCreateRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pgroup_createGroup",
		Params: []pentePrivacyGroupInput{
			{
				Domain:  "pente",
				Members: members,
				Name:    groupName,
				Configuration: map[string]interface{}{
					"endorsementType": "group_scoped_identities",
					"evmVersion":      "shanghai",
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var createResp pGroupCreateResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if createResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", createResp.Error.Message)
	}
	if createResp.Result == nil {
		return nil, fmt.Errorf("empty result from pgroup_createGroup: %s", string(respBody))
	}

	return &createResp, nil
}

// spokeName returns the SPOKE env var or defaults to spoke-a
func spokeName() string {
	if s := os.Getenv("SPOKE"); s != "" {
		return s
	}
	return "spoke-a"
}

// bilateralMembers returns the identities used to create the default bilateral context.
// A custom second member can be provided via PENTE_COUNTERPARTY_IDENTITY.
func bilateralMembers(spoke string) []string {
	cb := fmt.Sprintf("funded_operator@%s-cb", spoke)
	if cp := os.Getenv("PENTE_COUNTERPARTY_IDENTITY"); cp != "" {
		return []string{cb, cp}
	}

	bankSuffix := "bank-a"
	switch spoke {
	case "spoke-b":
		bankSuffix = "bank-b"
	}

	counterparty := fmt.Sprintf("funded_operator@%s-%s", spoke, bankSuffix)
	return []string{cb, counterparty}
}

// pollTxReceipt polls ptx_getTransactionFull until receipt is available.
func pollTxReceipt(paladinURL, txId string) (string, error) {
	maxAttempts := 60
	for attempt := 0; attempt < maxAttempts; attempt++ {
		reqBody := ptxGetTxRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "ptx_getTransactionFull",
			Params:  []string{txId},
		}

		body, _ := json.Marshal(reqBody)
		resp, err := http.Post(paladinURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("ptx_getTransactionFull: %w", err)
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

// appendEnvFile appends key=value pairs to an env file.
// Kept as shared helper for other Pente script tests.
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
