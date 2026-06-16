// SPDX-License-Identifier: Apache-2.0

package besu

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestFiatClient_GetFiatBalance(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	fiatAddress := common.HexToAddress("0x00000000000000000000000000000000000000ff")

	var capturedMethod string
	var capturedFrom string
	var capturedTo string
	var capturedData string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req struct {
			JSONRPC string            `json:"jsonrpc"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
			ID      json.RawMessage   `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		capturedMethod = req.Method
		if req.Method != "eth_call" {
			t.Fatalf("expected eth_call, got %s", req.Method)
		}
		if len(req.Params) < 1 {
			t.Fatal("expected eth_call params")
		}

		var call map[string]string
		if err := json.Unmarshal(req.Params[0], &call); err != nil {
			t.Fatalf("unmarshal call params: %v", err)
		}

		capturedFrom = call["from"]
		capturedTo = call["to"]
		capturedData = call["data"]
		if capturedData == "" {
			capturedData = call["input"]
		}

		result := hexutil.Encode(common.LeftPadBytes(big.NewInt(1250000).Bytes(), 32))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(req.ID),
			"result":  result,
		})
	}))
	defer server.Close()

	ethClient, err := ethclient.Dial(server.URL)
	if err != nil {
		t.Fatalf("dial test rpc: %v", err)
	}
	defer ethClient.Close()

	parsedABI, err := abi.JSON(strings.NewReader(fiatABIJSON))
	if err != nil {
		t.Fatalf("parse fiat ABI: %v", err)
	}

	client := &FiatClient{
		ethClient:   ethClient,
		fiatAddress: fiatAddress,
		fiatABI:     parsedABI,
		privateKey:  privateKey,
		fromAddress: fromAddress,
		chainID:     big.NewInt(1337),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	balance, err := client.GetFiatBalance(context.Background())
	if err != nil {
		t.Fatalf("GetFiatBalance: %v", err)
	}
	if balance != "1250000" {
		t.Fatalf("expected 1250000, got %s", balance)
	}
	if capturedMethod != "eth_call" {
		t.Fatalf("expected method eth_call, got %s", capturedMethod)
	}
	if common.HexToAddress(capturedFrom) != fromAddress {
		t.Fatalf("expected from %s, got %s", fromAddress.Hex(), capturedFrom)
	}
	if common.HexToAddress(capturedTo) != fiatAddress {
		t.Fatalf("expected to %s, got %s", fiatAddress.Hex(), capturedTo)
	}

	expectedData, err := parsedABI.Pack("balanceOf", fromAddress)
	if err != nil {
		t.Fatalf("pack expected balanceOf data: %v", err)
	}
	if capturedData != hexutil.Encode(expectedData) {
		t.Fatalf("expected data %s, got %s", hexutil.Encode(expectedData), capturedData)
	}
}
