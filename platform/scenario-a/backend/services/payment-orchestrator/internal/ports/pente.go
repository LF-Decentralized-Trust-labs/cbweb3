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
