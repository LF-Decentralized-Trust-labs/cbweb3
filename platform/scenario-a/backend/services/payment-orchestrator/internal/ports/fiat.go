// SPDX-License-Identifier: Apache-2.0

package ports

import "context"

// FiatTokenPort abstracts on-chain ERC-20 operations against the
// FiatCentralBankMoney (fCeBM) contract deployed on the spoke's Besu chain.
type FiatTokenPort interface {
	Mint(ctx context.Context, toAddress string, amount string) (txHash string, err error)
	Burn(ctx context.Context, fromAddress string, amount string) (txHash string, err error)
	BalanceOf(ctx context.Context, address string) (string, error)
	GetFiatBalance(ctx context.Context) (string, error)

	// Decimals and Symbol describe the token's scale and code, read from the
	// contract. Added for ADR-009: before it, nothing in Scenario A knew the scale,
	// so the portal printed the raw base-unit integer as if it were a currency
	// amount — a displayed "1,000 fCeBM" was 1000 wei of an 18-decimal token.
	Decimals(ctx context.Context) (uint8, error)
	Symbol(ctx context.Context) (string, error)
}
