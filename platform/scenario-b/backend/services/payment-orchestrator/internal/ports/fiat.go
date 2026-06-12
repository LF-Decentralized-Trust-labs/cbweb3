// SPDX-License-Identifier: Apache-2.0

package ports

import "context"

// TCeBMPort abstracts on-chain ERC-20 operations against the
// TokenizedCentralBankMoney (tCeBM) contract deployed on the spoke's Besu chain.
type TCeBMPort interface {
	Decimals(ctx context.Context) (uint8, error)
	Mint(ctx context.Context, toAddress string, amount string) (txHash string, err error)
	Burn(ctx context.Context, fromAddress string, amount string) (txHash string, err error)
	BalanceOf(ctx context.Context, address string) (string, error)
	GetBalance(ctx context.Context) (string, error)
}
