// SPDX-License-Identifier: Apache-2.0

package ports

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// FXProposalParams contains the parameters for proposing an FX agreement on-chain.
type FXProposalParams struct {
	TradeID         [32]byte
	Originator      common.Address // only used in ProposeOnBehalf
	CounterpartyB   common.Address
	SettlementAgent common.Address
	Custodian       common.Address
	Beneficiary     common.Address
	OriginAmount    *big.Int
	CounterAmount   *big.Int
	OriginCurrency  [32]byte
	CounterCurrency [32]byte
	Rate            *big.Int
	ExpiryDate      *big.Int
}

// FXAgreementContractPort abstracts on-chain FXAgreement operations against
// FXAgreement.sol deployed on the spoke's Besu chain.
type FXAgreementContractPort interface {
	Propose(ctx context.Context, params FXProposalParams) (txHash string, err error)
	ProposeOnBehalf(ctx context.Context, params FXProposalParams) (txHash string, err error)
	Accept(ctx context.Context, tradeID [32]byte) (txHash string, err error)
	AcceptOnBehalf(ctx context.Context, tradeID [32]byte) (txHash string, err error)
	Reject(ctx context.Context, tradeID [32]byte) (txHash string, err error)
	RejectOnBehalf(ctx context.Context, tradeID [32]byte) (txHash string, err error)
	Cancel(ctx context.Context, tradeID [32]byte) (txHash string, err error)
	Settle(ctx context.Context, tradeID [32]byte) (txHash string, err error)
}
