// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func newKeyHex(t *testing.T) string {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return hex.EncodeToString(gethcrypto.FromECDSA(key))
}

func TestNewSigner(t *testing.T) {
	keyHex := newKeyHex(t)

	t.Run("success", func(t *testing.T) {
		s, err := NewSigner("0x"+keyHex, big.NewInt(1337))
		if err != nil {
			t.Fatalf("NewSigner: %v", err)
		}
		if (s.Address() == common.Address{}) {
			t.Fatal("expected non-zero address")
		}
		if s.ChainID().Cmp(big.NewInt(1337)) != 0 {
			t.Fatalf("chainID=%s", s.ChainID())
		}
		// ChainID returns a copy; mutating it must not affect the signer.
		got := s.ChainID()
		got.SetInt64(99)
		if s.ChainID().Cmp(big.NewInt(1337)) != 0 {
			t.Fatal("ChainID must return a defensive copy")
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		if _, err := NewSigner("  ", big.NewInt(1)); err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("nil_chain_id", func(t *testing.T) {
		if _, err := NewSigner(keyHex, nil); err == nil {
			t.Fatal("expected error for nil chainID")
		}
	})

	t.Run("non_positive_chain_id", func(t *testing.T) {
		if _, err := NewSigner(keyHex, big.NewInt(0)); err == nil {
			t.Fatal("expected error for zero chainID")
		}
	})

	t.Run("invalid_key", func(t *testing.T) {
		if _, err := NewSigner("zzzz", big.NewInt(1)); err == nil {
			t.Fatal("expected error for invalid key")
		}
	})
}

func TestSigner_TransactOpts(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	ctx := context.Background()
	opts, err := s.TransactOpts(ctx)
	if err != nil {
		t.Fatalf("TransactOpts: %v", err)
	}
	if opts.From != s.Address() {
		t.Fatalf("opts.From=%s want %s", opts.From.Hex(), s.Address().Hex())
	}
	if opts.Context != ctx {
		t.Fatal("opts.Context not set")
	}
}

func TestParseABI(t *testing.T) {
	const raw = `[{"type":"function","name":"foo","inputs":[],"outputs":[{"type":"uint256"}]}]`
	t.Run("valid", func(t *testing.T) {
		parsed, err := ParseABI("  " + raw + "  ")
		if err != nil {
			t.Fatalf("ParseABI: %v", err)
		}
		if _, ok := parsed.Methods["foo"]; !ok {
			t.Fatal("method foo missing")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := ParseABI("not json"); err == nil {
			t.Fatal("expected error for invalid ABI")
		}
	})
}

func TestDial_Errors(t *testing.T) {
	t.Run("empty_url", func(t *testing.T) {
		if _, err := Dial(context.Background(), "", time.Second); err == nil {
			t.Fatal("expected error for empty URL")
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		// ethclient.DialContext for an HTTP URL is lazy and may not error here;
		// use an obviously malformed scheme to force a dial error.
		if _, err := Dial(context.Background(), "://bad", 100*time.Millisecond); err == nil {
			t.Fatal("expected error for malformed URL")
		}
	})
}

func TestSigner_NonceInit(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if s.nonceInit {
		t.Fatal("nonceInit should start false; first signAndSend seeds it from PendingNonceAt")
	}
}

func TestIsNonceTooLow(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"nonce too low", true},
		{"nonce too high", true},
		{"replacement transaction underpriced", true},
		{"NONCE TOO LOW", true}, // case-insensitive
		{"insufficient funds", false},
		{"known transaction", false},
		{"gas limit exceeded", false},
	}
	for _, tc := range tests {
		if got := isNonceTooLow(fmt.Errorf("%s", tc.msg)); got != tc.want {
			t.Errorf("isNonceTooLow(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestCopyOutput(t *testing.T) {
	t.Run("big_int", func(t *testing.T) {
		var dst big.Int
		if err := copyOutput(big.NewInt(42), &dst); err != nil || dst.Int64() != 42 {
			t.Fatalf("err=%v dst=%s", err, dst.String())
		}
	})
	t.Run("bool", func(t *testing.T) {
		var dst bool
		if err := copyOutput(true, &dst); err != nil || !dst {
			t.Fatalf("err=%v dst=%v", err, dst)
		}
	})
	t.Run("address", func(t *testing.T) {
		var dst common.Address
		src := common.HexToAddress("0x1111111111111111111111111111111111111111")
		if err := copyOutput(src, &dst); err != nil || dst != src {
			t.Fatalf("err=%v dst=%s", err, dst.Hex())
		}
	})
	t.Run("bytes32", func(t *testing.T) {
		var dst [32]byte
		var src [32]byte
		src[0] = 0xAB
		if err := copyOutput(src, &dst); err != nil || dst != src {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("string", func(t *testing.T) {
		var dst string
		if err := copyOutput("hi", &dst); err != nil || dst != "hi" {
			t.Fatalf("err=%v dst=%q", err, dst)
		}
	})
	t.Run("type_mismatch", func(t *testing.T) {
		var dst big.Int
		if err := copyOutput("not-a-bigint", &dst); err == nil {
			t.Fatal("expected type mismatch error")
		}
	})
	t.Run("unsupported_dst", func(t *testing.T) {
		var dst float64
		if err := copyOutput(1.0, &dst); err == nil {
			t.Fatal("expected unsupported type error")
		}
	})
}

// --- raw-input submission ---
//
// Three payment-orchestrator clients (tCeBM, fiat, FX agreement) each built their own transactor
// from the same operator key and fetched PendingNonceAt themselves, bypassing this package's
// serialized counter. With the same key on the same chain, two concurrent submissions could claim
// the same nonce and one would be replaced — the failure this Signer exists to prevent, and the
// same one that motivated giving the CB's relayer its own identity.
//
// They pack their own calldata, so converging them needs a raw-input entry point rather than a
// method+args one. These tests pin the contract of that entry point; the nonce serialization itself
// is exercised by the shared path they now share.

func TestSubmitRawTxReceipt_RequiresASigner(t *testing.T) {
	if _, _, err := SubmitRawTxReceipt(context.Background(), nil, nil,
		common.HexToAddress("0x1"), []byte{0x01}, "mint"); err == nil {
		t.Fatal("expected a nil signer to be refused rather than panicking")
	}
}

func TestSubmitRawTxReceipt_RequiresInput(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	if _, _, err := SubmitRawTxReceipt(context.Background(), nil, s,
		common.HexToAddress("0x1"), nil, "mint"); err == nil {
		t.Fatal("expected empty calldata to be refused: it would send a bare value transfer")
	}
}
