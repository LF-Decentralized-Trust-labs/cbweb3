// SPDX-License-Identifier: Apache-2.0

package ports

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// FXRouting is the cross-spoke routing metadata stored on-chain alongside the FX agreement
// (inside the private Pente group). Spoke IDs + Paladin identity strings let the coordinating
// central bank read the full deal and the relay route the destination leg by identity — robust
// to the shared-key address collision in local dev.
type FXRouting struct {
	SourceSpokeID     string
	DestSpokeID       string
	OriginatorID      string
	CounterpartyID    string
	SettlementAgentID string
	CustodianID       string
	BeneficiaryID     string
	SourceReceiverID  string
	DestReceiverID    string
	// TradeRef is the off-chain trade reference (UUID). The on-chain bytes32 key is
	// sha256(TradeRef); storing the ref lets the CB recover the id from chain state.
	TradeRef string
}

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
	Routing         FXRouting
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
