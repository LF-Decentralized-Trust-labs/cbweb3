// swap_lookup.go verifies executed AMM swaps on the Hub chain (R2-CR-6).
//
// A bridge-out must never trust relay-supplied JSON for amounts or addresses: the
// authoritative record is the swap transaction itself. SwapByTxHash fetches the
// receipt and ParseSwapLogs extracts, from the trusted AMM contract's own events,
// what was swapped and where the output tokens actually went.
package amm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	logSwapTopic  = crypto.Keccak256Hash([]byte("LogSwap(address,address,address,uint256,uint256)"))
	transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
)

// VerifiedSwap is the on-chain truth about an executed AMM swap.
type VerifiedSwap struct {
	User      string // msg.sender of the swap (LogSwap.user)
	TokenIn   string // ERC-20 paid into the pool
	TokenOut  string // ERC-20 received from the pool
	AmountIn  string // gross input amount (fee included), decimal string
	AmountOut string // output amount, decimal string
	Recipient string // address the tokenOut Transfer was sent to (the swap's `to`)
}

// SwapByTxHash loads the receipt for txHash from the Hub and returns the verified
// swap facts. It fails if the transaction is missing, reverted, or does not contain
// a LogSwap emitted by this client's configured AMM contract.
func (c *Client) SwapByTxHash(ctx context.Context, txHash string) (*VerifiedSwap, error) {
	h := strings.TrimSpace(txHash)
	if !strings.HasPrefix(h, "0x") || len(h) != 66 {
		return nil, fmt.Errorf("amm: invalid swap tx hash %q", txHash)
	}
	receipt, err := c.ec.TransactionReceipt(ctx, common.HexToHash(h))
	if err != nil {
		return nil, fmt.Errorf("amm: swap tx %s not found on Hub: %w", h, err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("amm: swap tx %s reverted", h)
	}
	return ParseSwapLogs(receipt.Logs, c.contract)
}

// ParseSwapLogs extracts the swap facts from a receipt's logs.
//
// Only a LogSwap emitted by the trusted AMM address is accepted — any contract can
// emit an event with the same signature, so the emitter check is load-bearing. The
// output recipient is established from the tokenOut ERC-20 Transfer sent by the AMM
// in the same transaction, cross-checked against LogSwap's amountOut.
func ParseSwapLogs(logs []*types.Log, ammAddress common.Address) (*VerifiedSwap, error) {
	var swap *VerifiedSwap
	var amountOut *big.Int
	for _, lg := range logs {
		if lg.Address != ammAddress || len(lg.Topics) != 4 || lg.Topics[0] != logSwapTopic {
			continue
		}
		if len(lg.Data) != 64 {
			return nil, fmt.Errorf("amm: malformed LogSwap data (%d bytes)", len(lg.Data))
		}
		amountIn := new(big.Int).SetBytes(lg.Data[:32])
		amountOut = new(big.Int).SetBytes(lg.Data[32:])
		swap = &VerifiedSwap{
			User:      common.BytesToAddress(lg.Topics[1].Bytes()).Hex(),
			TokenIn:   common.BytesToAddress(lg.Topics[2].Bytes()).Hex(),
			TokenOut:  common.BytesToAddress(lg.Topics[3].Bytes()).Hex(),
			AmountIn:  amountIn.String(),
			AmountOut: amountOut.String(),
		}
		break
	}
	if swap == nil {
		return nil, errors.New("amm: no LogSwap emitted by the trusted AMM contract in this transaction")
	}

	tokenOut := common.HexToAddress(swap.TokenOut)
	for _, lg := range logs {
		if lg.Address != tokenOut || len(lg.Topics) != 3 || lg.Topics[0] != transferTopic {
			continue
		}
		from := common.BytesToAddress(lg.Topics[1].Bytes())
		if from != ammAddress {
			continue
		}
		value := new(big.Int).SetBytes(lg.Data)
		if value.Cmp(amountOut) != 0 {
			continue
		}
		swap.Recipient = common.BytesToAddress(lg.Topics[2].Bytes()).Hex()
		return swap, nil
	}
	return nil, errors.New("amm: no tokenOut Transfer from the AMM matching LogSwap amountOut")
}
