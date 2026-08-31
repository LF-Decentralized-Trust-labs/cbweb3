// SPDX-License-Identifier: Apache-2.0

package ports

import "context"

// PenteContextRequest carries the minimum business identifiers needed to
// create or reuse a bilateral private context for an FX agreement.
type PenteContextRequest struct {
	TradeID      string
	Originator   string
	Counterparty string
}

// PenteContextResult identifies the bilateral private context and contract.
type PenteContextResult struct {
	GroupID         string
	ContractAddress string
}

// PenteFXAgreementState is a lightweight representation of an agreement state
// returned by the private Pente gateway.
type PenteFXAgreementState struct {
	TradeID string
	State   string
	TxHash  string
}

type penteFXTargetKey struct{}

// PenteFXTarget carries the bilateral private context metadata used by Pente
// FXAgreement actions.
type PenteFXTarget struct {
	GroupID         string
	ContractAddress string
}

// WithPenteFXTarget binds contract/group metadata to ctx for a subsequent
// Pente FXAgreement call.
func WithPenteFXTarget(ctx context.Context, target PenteFXTarget) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, penteFXTargetKey{}, target)
}

// PenteFXTargetFromContext extracts PenteFXTarget from ctx when present.
func PenteFXTargetFromContext(ctx context.Context) (PenteFXTarget, bool) {
	target, ok := ctx.Value(penteFXTargetKey{}).(PenteFXTarget)
	return target, ok
}

// PenteClientPort abstracts bilateral private-context lifecycle for FX flows.
type PenteClientPort interface {
	EnsureFXContext(ctx context.Context, req PenteContextRequest) (*PenteContextResult, error)
}

// PenteGroup is a bilateral privacy group the local node is a member of. ContractAddress is the
// group's base-ledger privacy contract — it equals the `source` field of every transaction
// receipt produced inside the group, which is how a receipt is mapped back to its group.
type PenteGroup struct {
	ID              string
	Name            string
	Members         []string
	ContractAddress string
}

// PenteReceiptRef is a lightweight transaction-receipt reference from ptx_queryTransactionReceipts.
type PenteReceiptRef struct {
	ID       string
	Source   string // base-ledger privacy-group contract address (maps to PenteGroup.ContractAddress)
	Sequence int64
	TxHash   string
}

// PenteLog is a decoded EVM log from a Pente domain receipt.
type PenteLog struct {
	Address string
	Topics  []string
	Data    string
}

// PenteFXAgreementFull is the complete on-chain FX agreement (financial terms + routing), read
// from a private group via getAgreement + getRouting. State is the raw on-chain enum ordinal.
type PenteFXAgreementFull struct {
	TradeIDHex      string
	Originator      string // in-group EVM address (hex)
	CounterpartyB   string
	SettlementAgent string
	Custodian       string
	Beneficiary     string
	OriginAmount    string // integer string
	CounterAmount   string
	OriginCurrency  string // decoded ISO code (e.g. "BRL")
	CounterCurrency string
	Rate            string // 1e18 fixed-point integer string
	ExpiryDate      uint64
	State           int
	Routing         FXRouting
}

// FXChainReaderPort is the read side of a Pente node used by the CB indexer to project on-chain
// FX agreements from every bilateral group the node belongs to. All data comes from chain state —
// no out-of-band context file is required.
type FXChainReaderPort interface {
	// QueryGroups lists the bilateral privacy groups the node is a member of.
	QueryGroups(ctx context.Context, limit int) ([]PenteGroup, error)
	// ListReceipts returns Pente transaction receipts with sequence > afterSequence, ascending.
	ListReceipts(ctx context.Context, afterSequence int64, limit int) ([]PenteReceiptRef, error)
	// DomainReceiptLogs returns the decoded EVM logs of a Pente transaction's domain receipt.
	DomainReceiptLogs(ctx context.Context, receiptID string) ([]PenteLog, error)
	// ReadFXAgreement reads the full agreement (getAgreement + getRouting) for a trade in a group.
	ReadFXAgreement(ctx context.Context, target PenteFXTarget, tradeIDHex string) (*PenteFXAgreementFull, error)
}
