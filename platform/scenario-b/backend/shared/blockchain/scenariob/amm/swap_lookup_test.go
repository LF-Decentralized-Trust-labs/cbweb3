// SPDX-License-Identifier: Apache-2.0

package amm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ammAddr   = common.HexToAddress("0x9fbda871d559710256a2502a2517b794b482db40")
	tokenIn   = common.HexToAddress("0x1111111111111111111111111111111111111111")
	tokenOut  = common.HexToAddress("0x4e72770760c011647d4873f60a3cf6cdea896cd8")
	swapUser  = common.HexToAddress("0x2222222222222222222222222222222222222222")
	recipient = common.HexToAddress("0xaaaa770760c011647d4873f60a3cf6cdea896cd8")
)

func uint256Data(vals ...*big.Int) []byte {
	out := make([]byte, 0, 32*len(vals))
	for _, v := range vals {
		out = append(out, common.LeftPadBytes(v.Bytes(), 32)...)
	}
	return out
}

func logSwapLog(emitter common.Address, amountIn, amountOut *big.Int) *types.Log {
	return &types.Log{
		Address: emitter,
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("LogSwap(address,address,address,uint256,uint256)")),
			common.BytesToHash(swapUser.Bytes()),
			common.BytesToHash(tokenIn.Bytes()),
			common.BytesToHash(tokenOut.Bytes()),
		},
		Data: uint256Data(amountIn, amountOut),
	}
}

func transferLog(token, from, to common.Address, value *big.Int) *types.Log {
	return &types.Log{
		Address: token,
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")),
			common.BytesToHash(from.Bytes()),
			common.BytesToHash(to.Bytes()),
		},
		Data: uint256Data(value),
	}
}

// A receipt with a LogSwap from the AMM and the matching tokenOut Transfer yields the
// verified output: token, on-chain amount, and the address the tokens actually went to.
func TestParseSwapLogs_HappyPath(t *testing.T) {
	logs := []*types.Log{
		transferLog(tokenIn, swapUser, ammAddr, big.NewInt(40)), // input leg, must be ignored
		logSwapLog(ammAddr, big.NewInt(40), big.NewInt(60)),
		transferLog(tokenOut, ammAddr, recipient, big.NewInt(60)), // output leg
	}

	got, err := ParseSwapLogs(logs, ammAddr)
	if err != nil {
		t.Fatalf("ParseSwapLogs: %v", err)
	}
	if got.TokenOut != tokenOut.Hex() {
		t.Errorf("TokenOut = %s, want %s", got.TokenOut, tokenOut.Hex())
	}
	if got.AmountOut != "60" {
		t.Errorf("AmountOut = %s, want 60", got.AmountOut)
	}
	if got.AmountIn != "40" {
		t.Errorf("AmountIn = %s, want 40", got.AmountIn)
	}
	if got.Recipient != recipient.Hex() {
		t.Errorf("Recipient = %s, want %s", got.Recipient, recipient.Hex())
	}
}

// A LogSwap emitted by a contract other than the trusted AMM must not verify:
// an attacker can deploy a contract that emits identical events.
func TestParseSwapLogs_SpoofedEmitterRejected(t *testing.T) {
	spoofer := common.HexToAddress("0xbadbadbadbadbadbadbadbadbadbadbadbadbad0")
	logs := []*types.Log{
		logSwapLog(spoofer, big.NewInt(40), big.NewInt(60)),
		transferLog(tokenOut, spoofer, recipient, big.NewInt(60)),
	}

	if _, err := ParseSwapLogs(logs, ammAddr); err == nil {
		t.Fatal("expected error for LogSwap from non-AMM emitter, got nil")
	}
}

// A transaction without a LogSwap (e.g. a plain transfer) must not verify.
func TestParseSwapLogs_NoSwapEventRejected(t *testing.T) {
	logs := []*types.Log{
		transferLog(tokenOut, ammAddr, recipient, big.NewInt(60)),
	}

	if _, err := ParseSwapLogs(logs, ammAddr); err == nil {
		t.Fatal("expected error for receipt without LogSwap, got nil")
	}
}

// The output Transfer must come from the AMM on the tokenOut contract and match the
// LogSwap amount; otherwise the recipient cannot be established.
func TestParseSwapLogs_MissingOutputTransferRejected(t *testing.T) {
	logs := []*types.Log{
		logSwapLog(ammAddr, big.NewInt(40), big.NewInt(60)),
		// Transfer of the right amount but on the wrong token contract.
		transferLog(tokenIn, ammAddr, recipient, big.NewInt(60)),
	}

	if _, err := ParseSwapLogs(logs, ammAddr); err == nil {
		t.Fatal("expected error for missing tokenOut Transfer, got nil")
	}
}
