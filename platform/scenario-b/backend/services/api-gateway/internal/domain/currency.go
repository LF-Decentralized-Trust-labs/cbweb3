// SPDX-License-Identifier: Apache-2.0

package domain

// CurrencyEntry is the in-memory representation of a registered currency
// on the hub CurrencyRegistry contract (006-hub-currency-registry).
// Read directly from on-chain via getAllCurrencies() — no DB table.
type CurrencyEntry struct {
	Symbol       string // e.g. "BRL"
	CountryName  string // e.g. "Brazil"
	TokenAddress string // on-chain tCeBM address
	ProposerCB   string // human-readable CB identifier, e.g. "central_bank_a"
}
