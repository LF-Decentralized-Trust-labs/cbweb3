// SPDX-License-Identifier: Apache-2.0

// Package payment provides a gRPC client adapter for the payment-orchestrator service.
package payment

import (
	"context"
	"time"

	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCAdapter is the api-gateway adapter for the payment-orchestrator gRPC service.
type GRPCAdapter struct {
	conn *grpc.ClientConn
	cc   pb.PaymentOrchestratorServiceClient
}

// NewGRPCAdapter connects to the payment-orchestrator and returns a GRPCAdapter.
func NewGRPCAdapter(address string, timeout time.Duration) (*GRPCAdapter, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:staticcheck
	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &GRPCAdapter{conn: conn, cc: pb.NewPaymentOrchestratorServiceClient(conn)}, nil
}

// Close releases the underlying gRPC connection.
func (a *GRPCAdapter) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// --- HTLC ---

type LockHTLCResult struct {
	ContractID string `json:"contract_id"`
	HashLock   string `json:"hash_lock"`
	HTLCTxHash string `json:"htlc_tx_hash,omitempty"`
	ZetoTxHash string `json:"zeto_tx_hash,omitempty"`
	Secret     string `json:"secret,omitempty"` // one-time; store securely — never re-exposed after this response
}

func (a *GRPCAdapter) LockHTLC(ctx context.Context, agreementID, receiver, amount string, timeLock uint64) (*LockHTLCResult, error) {
	resp, err := a.cc.LockHTLC(ctx, &pb.LockHTLCRequest{
		AgreementId: agreementID,
		Receiver:    receiver,
		Amount:      amount,
		TimeLock:    timeLock,
	})
	if err != nil {
		return nil, err
	}
	return &LockHTLCResult{
		ContractID: resp.ContractId,
		HashLock:   resp.HashLock,
		HTLCTxHash: resp.HtlcTxHash,
		ZetoTxHash: resp.ZetoTxHash,
		Secret:     resp.Secret,
	}, nil
}

func (a *GRPCAdapter) LockHTLCWithHashLock(ctx context.Context, agreementID, receiver, amount string, timeLock uint64, hashLock string) (*LockHTLCResult, error) {
	resp, err := a.cc.LockHTLCWithHashLock(ctx, &pb.LockHTLCWithHashLockRequest{
		AgreementId: agreementID,
		Receiver:    receiver,
		Amount:      amount,
		TimeLock:    timeLock,
		HashLock:    hashLock,
	})
	if err != nil {
		return nil, err
	}
	return &LockHTLCResult{
		ContractID: resp.ContractId,
		HashLock:   resp.HashLock,
		HTLCTxHash: resp.HtlcTxHash,
		ZetoTxHash: resp.ZetoTxHash,
	}, nil
}

type SettleHTLCResult struct {
	HTLCTxHash string `json:"htlc_tx_hash,omitempty"`
	ZetoTxHash string `json:"zeto_tx_hash,omitempty"`
}

func (a *GRPCAdapter) SettleHTLC(ctx context.Context, contractID, secret string) (*SettleHTLCResult, error) {
	resp, err := a.cc.SettleHTLC(ctx, &pb.SettleHTLCRequest{
		ContractId: contractID,
		Secret:     secret,
	})
	if err != nil {
		return nil, err
	}
	return &SettleHTLCResult{
		HTLCTxHash: resp.HtlcTxHash,
		ZetoTxHash: resp.ZetoTxHash,
	}, nil
}

type RefundHTLCResult struct {
	HTLCTxHash string `json:"htlc_tx_hash,omitempty"`
	ZetoTxHash string `json:"zeto_tx_hash,omitempty"`
}

func (a *GRPCAdapter) RefundHTLC(ctx context.Context, contractID string) (*RefundHTLCResult, error) {
	resp, err := a.cc.RefundHTLC(ctx, &pb.RefundHTLCRequest{
		ContractId: contractID,
	})
	if err != nil {
		return nil, err
	}
	return &RefundHTLCResult{
		HTLCTxHash: resp.HtlcTxHash,
		ZetoTxHash: resp.ZetoTxHash,
	}, nil
}

type HTLCStatus struct {
	ContractID         string `json:"contract_id"`
	Sender             string `json:"sender"`
	Receiver           string `json:"receiver"`
	HashLock           string `json:"hash_lock"`
	TimeLock           uint64 `json:"time_lock"`
	ZetoLockRef        string `json:"zeto_lock_ref"`
	State              string `json:"state"`
	CounterpartyLocked bool   `json:"counterparty_locked"`
}

func (a *GRPCAdapter) GetHTLCStatus(ctx context.Context, contractID string) (*HTLCStatus, error) {
	resp, err := a.cc.GetHTLCStatus(ctx, &pb.GetHTLCStatusRequest{
		ContractId: contractID,
	})
	if err != nil {
		return nil, err
	}
	return lockToStatus(resp.Lock), nil
}

func (a *GRPCAdapter) SearchHTLC(ctx context.Context, agreementID, sender, receiver, state string) ([]HTLCStatus, error) {
	resp, err := a.cc.SearchHTLC(ctx, &pb.SearchHTLCRequest{
		AgreementId: agreementID,
		Sender:      sender,
		Receiver:    receiver,
		State:       state,
	})
	if err != nil {
		return nil, err
	}
	result := make([]HTLCStatus, 0, len(resp.Locks))
	for _, l := range resp.Locks {
		result = append(result, *lockToStatus(l))
	}
	return result, nil
}

// --- Token ---

type TokenResult struct {
	TxHash string `json:"tx_hash"`
}

func (a *GRPCAdapter) MintToken(ctx context.Context, to, amount string) (*TokenResult, error) {
	resp, err := a.cc.MintToken(ctx, &pb.MintTokenRequest{To: to, Amount: amount})
	if err != nil {
		return nil, err
	}
	return &TokenResult{TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) BurnToken(ctx context.Context, from, amount string) (*TokenResult, error) {
	resp, err := a.cc.BurnToken(ctx, &pb.BurnTokenRequest{From: from, Amount: amount})
	if err != nil {
		return nil, err
	}
	return &TokenResult{TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) TransferToken(ctx context.Context, to, amount string) (*TokenResult, error) {
	resp, err := a.cc.TransferToken(ctx, &pb.TransferTokenRequest{To: to, Amount: amount})
	if err != nil {
		return nil, err
	}
	return &TokenResult{TxHash: resp.TxHash}, nil
}

type BalanceResult struct {
	Balance string `json:"balance"`
}

func (a *GRPCAdapter) GetBalance(ctx context.Context) (*BalanceResult, error) {
	resp, err := a.cc.GetBalance(ctx, &pb.GetBalanceRequest{})
	if err != nil {
		return nil, err
	}
	return &BalanceResult{Balance: resp.Balance}, nil
}

type FiatBalanceResult struct {
	Balance string `json:"balance"`
}

func (a *GRPCAdapter) GetFiatBalance(ctx context.Context) (*FiatBalanceResult, error) {
	resp, err := a.cc.GetFiatBalance(ctx, &pb.GetFiatBalanceRequest{})
	if err != nil {
		return nil, err
	}
	return &FiatBalanceResult{Balance: resp.Balance}, nil
}

func lockToStatus(l *pb.HTLCLock) *HTLCStatus {
	if l == nil {
		return &HTLCStatus{}
	}
	return &HTLCStatus{
		ContractID:         l.ContractId,
		Sender:             l.Sender,
		Receiver:           l.Receiver,
		HashLock:           l.HashLock,
		TimeLock:           l.TimeLock,
		ZetoLockRef:        l.ZetoLockRef,
		State:              l.State.String(),
		CounterpartyLocked: l.CounterpartyLocked,
	}
}

// --- Escrow: Deposits ---

type RegisterDepositResult struct {
	DepositID string `json:"deposit_id"`
}

func (a *GRPCAdapter) RegisterDeposit(ctx context.Context, besuAddr, paladinIdentity, amount string) (*RegisterDepositResult, error) {
	resp, err := a.cc.RegisterDeposit(ctx, &pb.RegisterDepositRequest{
		RequesterBesuAddress:     besuAddr,
		RequesterPaladinIdentity: paladinIdentity,
		Amount:                   amount,
	})
	if err != nil {
		return nil, err
	}
	return &RegisterDepositResult{DepositID: resp.DepositId}, nil
}

func (a *GRPCAdapter) ApproveDeposit(ctx context.Context, depositID string) error {
	_, err := a.cc.ApproveDeposit(ctx, &pb.ApproveDepositRequest{DepositId: depositID})
	return err
}

func (a *GRPCAdapter) RejectDeposit(ctx context.Context, depositID, reason string) error {
	_, err := a.cc.RejectDeposit(ctx, &pb.RejectDepositRequest{DepositId: depositID, Reason: reason})
	return err
}

type FiatExchangeResult struct {
	MintTxHash string `json:"mint_tx_hash"`
}

func (a *GRPCAdapter) RequestFiatExchange(ctx context.Context, depositID string) (*FiatExchangeResult, error) {
	resp, err := a.cc.RequestFiatExchange(ctx, &pb.RequestFiatExchangeRequest{DepositId: depositID})
	if err != nil {
		return nil, err
	}
	return &FiatExchangeResult{MintTxHash: resp.MintTxHash}, nil
}

type DepositRecord struct {
	ID                       string `json:"id"`
	RequesterID              string `json:"requester_id"`
	RequesterBesuAddress     string `json:"requester_besu_address"`
	RequesterPaladinIdentity string `json:"requester_paladin_identity"`
	// RequesterName is the resolved institution name for RequesterBesuAddress.
	// Populated by the api-gateway handler via the compliance participant registry;
	// empty when the address has no registered participant.
	RequesterName   string `json:"requester_name,omitempty"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	MintTxHash      string `json:"mint_tx_hash,omitempty"`
	RejectionReason string `json:"rejection_reason,omitempty"`
	CreatedAt       string `json:"created_at"`
}

func (a *GRPCAdapter) ListDeposits(ctx context.Context, requesterID string) ([]DepositRecord, error) {
	resp, err := a.cc.ListDeposits(ctx, &pb.ListDepositsRequest{RequesterId: requesterID})
	if err != nil {
		return nil, err
	}
	result := make([]DepositRecord, 0, len(resp.Deposits))
	for _, d := range resp.Deposits {
		result = append(result, DepositRecord{
			ID:                       d.Id,
			RequesterID:              d.RequesterId,
			RequesterBesuAddress:     d.RequesterBesuAddress,
			RequesterPaladinIdentity: d.RequesterPaladinIdentity,
			Amount:                   d.Amount,
			Status:                   d.Status.String(),
			MintTxHash:               d.MintTxHash,
			RejectionReason:          d.RejectionReason,
			CreatedAt:                d.CreatedAt,
		})
	}
	return result, nil
}

// --- Escrow: Tokenization ---

type RequestEscrowResult struct {
	EscrowID string `json:"escrow_id"`
}

func (a *GRPCAdapter) RequestEscrow(ctx context.Context, besuAddr, paladinIdentity, amount string) (*RequestEscrowResult, error) {
	resp, err := a.cc.RequestEscrow(ctx, &pb.RequestEscrowRequest{
		RequesterBesuAddress:     besuAddr,
		RequesterPaladinIdentity: paladinIdentity,
		Amount:                   amount,
	})
	if err != nil {
		return nil, err
	}
	return &RequestEscrowResult{EscrowID: resp.EscrowId}, nil
}

type ApproveEscrowResult struct {
	BurnTxHash string `json:"burn_tx_hash"`
	MintTxHash string `json:"mint_tx_hash"`
}

func (a *GRPCAdapter) ApproveEscrow(ctx context.Context, escrowID string) (*ApproveEscrowResult, error) {
	resp, err := a.cc.ApproveEscrow(ctx, &pb.ApproveEscrowRequest{EscrowId: escrowID})
	if err != nil {
		return nil, err
	}
	return &ApproveEscrowResult{BurnTxHash: resp.BurnTxHash, MintTxHash: resp.MintTxHash}, nil
}

func (a *GRPCAdapter) RejectEscrow(ctx context.Context, escrowID, reason string) error {
	_, err := a.cc.RejectEscrow(ctx, &pb.RejectEscrowRequest{EscrowId: escrowID, Reason: reason})
	return err
}

type EscrowRecord struct {
	ID                       string `json:"id"`
	RequesterID              string `json:"requester_id"`
	RequesterBesuAddress     string `json:"requester_besu_address"`
	RequesterPaladinIdentity string `json:"requester_paladin_identity"`
	// RequesterName is the resolved institution name for RequesterBesuAddress
	// (see DepositRecord.RequesterName).
	RequesterName   string `json:"requester_name,omitempty"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	BurnTxHash      string `json:"burn_tx_hash,omitempty"`
	MintTxHash      string `json:"mint_tx_hash,omitempty"`
	RejectionReason string `json:"rejection_reason,omitempty"`
	CreatedAt       string `json:"created_at"`
}

func (a *GRPCAdapter) ListEscrows(ctx context.Context, requesterID string) ([]EscrowRecord, error) {
	resp, err := a.cc.ListEscrows(ctx, &pb.ListEscrowsRequest{RequesterId: requesterID})
	if err != nil {
		return nil, err
	}
	result := make([]EscrowRecord, 0, len(resp.Escrows))
	for _, e := range resp.Escrows {
		result = append(result, EscrowRecord{
			ID:                       e.Id,
			RequesterID:              e.RequesterId,
			RequesterBesuAddress:     e.RequesterBesuAddress,
			RequesterPaladinIdentity: e.RequesterPaladinIdentity,
			Amount:                   e.Amount,
			Status:                   e.Status.String(),
			BurnTxHash:               e.BurnTxHash,
			MintTxHash:               e.MintTxHash,
			RejectionReason:          e.RejectionReason,
			CreatedAt:                e.CreatedAt,
		})
	}
	return result, nil
}

// --- Escrow: Redeem ---

type InitiateZetoTransferResult struct {
	TxHash string `json:"tx_hash"`
}

func (a *GRPCAdapter) InitiateZetoTransfer(ctx context.Context, toIdentity, amount string) (*InitiateZetoTransferResult, error) {
	resp, err := a.cc.InitiateZetoTransfer(ctx, &pb.InitiateZetoTransferRequest{
		ToIdentity: toIdentity,
		Amount:     amount,
	})
	if err != nil {
		return nil, err
	}
	return &InitiateZetoTransferResult{TxHash: resp.TxHash}, nil
}

type RequestRedeemResult struct {
	RedeemID string `json:"redeem_id"`
}

func (a *GRPCAdapter) RequestRedeem(ctx context.Context, besuAddr, paladinIdentity, amount, zetoTxHash string) (*RequestRedeemResult, error) {
	resp, err := a.cc.RequestRedeem(ctx, &pb.RequestRedeemRequest{
		RequesterBesuAddress:     besuAddr,
		RequesterPaladinIdentity: paladinIdentity,
		Amount:                   amount,
		ZetoTransferTxHash:       zetoTxHash,
	})
	if err != nil {
		return nil, err
	}
	return &RequestRedeemResult{RedeemID: resp.RedeemId}, nil
}

type ApproveRedeemResult struct {
	FiatMintTxHash string `json:"fiat_mint_tx_hash"`
}

func (a *GRPCAdapter) ApproveRedeem(ctx context.Context, redeemID string) (*ApproveRedeemResult, error) {
	resp, err := a.cc.ApproveRedeem(ctx, &pb.ApproveRedeemRequest{RedeemId: redeemID})
	if err != nil {
		return nil, err
	}
	return &ApproveRedeemResult{FiatMintTxHash: resp.FiatMintTxHash}, nil
}

func (a *GRPCAdapter) RejectRedeem(ctx context.Context, redeemID, reason string) error {
	_, err := a.cc.RejectRedeem(ctx, &pb.RejectRedeemRequest{RedeemId: redeemID, Reason: reason})
	return err
}

type RedeemRecord struct {
	ID                       string `json:"id"`
	RequesterID              string `json:"requester_id"`
	RequesterBesuAddress     string `json:"requester_besu_address"`
	RequesterPaladinIdentity string `json:"requester_paladin_identity"`
	// RequesterName is the resolved institution name for RequesterBesuAddress
	// (see DepositRecord.RequesterName).
	RequesterName      string `json:"requester_name,omitempty"`
	Amount             string `json:"amount"`
	Status             string `json:"status"`
	ZetoTransferTxHash string `json:"zeto_transfer_tx_hash,omitempty"`
	FiatMintTxHash     string `json:"fiat_mint_tx_hash,omitempty"`
	RejectionReason    string `json:"rejection_reason,omitempty"`
	CreatedAt          string `json:"created_at"`
}

func (a *GRPCAdapter) ListRedeems(ctx context.Context, requesterID string) ([]RedeemRecord, error) {
	resp, err := a.cc.ListRedeems(ctx, &pb.ListRedeemsRequest{RequesterId: requesterID})
	if err != nil {
		return nil, err
	}
	result := make([]RedeemRecord, 0, len(resp.Redeems))
	for _, r := range resp.Redeems {
		result = append(result, RedeemRecord{
			ID:                       r.Id,
			RequesterID:              r.RequesterId,
			RequesterBesuAddress:     r.RequesterBesuAddress,
			RequesterPaladinIdentity: r.RequesterPaladinIdentity,
			Amount:                   r.Amount,
			Status:                   r.Status.String(),
			ZetoTransferTxHash:       r.ZetoTransferTxHash,
			FiatMintTxHash:           r.FiatMintTxHash,
			RejectionReason:          r.RejectionReason,
			CreatedAt:                r.CreatedAt,
		})
	}
	return result, nil
}

// --- FX Agreement ---

type FXAgreementResult struct {
	TradeID         string `json:"trade_id"`
	TxHash          string `json:"tx_hash,omitempty"`
	Originator      string `json:"originator,omitempty"`
	CounterpartyB   string `json:"counterparty_b,omitempty"`
	SettlementAgent string `json:"settlement_agent,omitempty"`
	Custodian       string `json:"custodian,omitempty"`
	Beneficiary     string `json:"beneficiary,omitempty"`
	OriginAmount    string `json:"origin_amount,omitempty"`
	CounterAmount   string `json:"counter_amount,omitempty"`
	OriginCurrency  string `json:"origin_currency,omitempty"`
	CounterCurrency string `json:"counter_currency,omitempty"`
	Rate            string `json:"rate,omitempty"`
	ExpiryDate      uint64 `json:"expiry_date,omitempty"`
	SourceSpokeID   string `json:"source_spoke_id,omitempty"`
	DestSpokeID     string `json:"dest_spoke_id,omitempty"`
	SourceReceiver  string `json:"source_receiver,omitempty"`
	DestReceiver    string `json:"dest_receiver,omitempty"`
	State           string `json:"state,omitempty"`
	GroupID         string `json:"group_id,omitempty"`
	ContractAddress string `json:"contract_address,omitempty"`
}

type FXAgreementEventResult struct {
	ID             int64  `json:"id"`
	TradeID        string `json:"trade_id"`
	FromState      string `json:"from_state"`
	ToState        string `json:"to_state"`
	Actor          string `json:"actor"`
	OccurredAtUnix int64  `json:"occurred_at_unix"`
	Notes          string `json:"notes,omitempty"`
	TxHash         string `json:"tx_hash,omitempty"`
	Source         string `json:"source"`
}

func (a *GRPCAdapter) ProposeFXAgreement(ctx context.Context, req *pb.ProposeFXAgreementRequest) (*FXAgreementResult, error) {
	resp, err := a.cc.ProposeFXAgreement(ctx, req)
	if err != nil {
		return nil, err
	}
	return &FXAgreementResult{TradeID: resp.TradeId, TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) AcceptFXAgreement(ctx context.Context, tradeID string, onBehalf bool) (*FXAgreementResult, error) {
	resp, err := a.cc.AcceptFXAgreement(ctx, &pb.AcceptFXAgreementRequest{TradeId: tradeID, OnBehalf: onBehalf})
	if err != nil {
		return nil, err
	}
	return &FXAgreementResult{TradeID: tradeID, TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) RejectFXAgreement(ctx context.Context, tradeID string, onBehalf bool) (*FXAgreementResult, error) {
	resp, err := a.cc.RejectFXAgreement(ctx, &pb.RejectFXAgreementRequest{TradeId: tradeID, OnBehalf: onBehalf})
	if err != nil {
		return nil, err
	}
	return &FXAgreementResult{TradeID: tradeID, TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) CancelFXAgreement(ctx context.Context, tradeID string) (*FXAgreementResult, error) {
	resp, err := a.cc.CancelFXAgreement(ctx, &pb.CancelFXAgreementRequest{TradeId: tradeID})
	if err != nil {
		return nil, err
	}
	return &FXAgreementResult{TradeID: tradeID, TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) SettleFXAgreement(ctx context.Context, tradeID string) (*FXAgreementResult, error) {
	resp, err := a.cc.SettleFXAgreement(ctx, &pb.SettleFXAgreementRequest{TradeId: tradeID})
	if err != nil {
		return nil, err
	}
	return &FXAgreementResult{TradeID: tradeID, TxHash: resp.TxHash}, nil
}

func (a *GRPCAdapter) GetFXAgreement(ctx context.Context, tradeID string) (*FXAgreementResult, error) {
	resp, err := a.cc.GetFXAgreement(ctx, &pb.GetFXAgreementRequest{TradeId: tradeID})
	if err != nil {
		return nil, err
	}
	return fxAgreementToResult(resp.Agreement), nil
}

func (a *GRPCAdapter) ListFXAgreements(ctx context.Context, counterparty, state string) ([]FXAgreementResult, error) {
	resp, err := a.cc.ListFXAgreements(ctx, &pb.ListFXAgreementsRequest{Counterparty: counterparty, State: state})
	if err != nil {
		return nil, err
	}
	result := make([]FXAgreementResult, 0, len(resp.Agreements))
	for _, ag := range resp.Agreements {
		result = append(result, *fxAgreementToResult(ag))
	}
	return result, nil
}

func (a *GRPCAdapter) ListFXAgreementEvents(ctx context.Context, tradeID string) ([]FXAgreementEventResult, error) {
	resp, err := a.cc.ListFXAgreementEvents(ctx, &pb.ListFXAgreementEventsRequest{TradeId: tradeID})
	if err != nil {
		return nil, err
	}
	result := make([]FXAgreementEventResult, 0, len(resp.Events))
	for _, ev := range resp.Events {
		result = append(result, *fxAgreementEventToResult(ev))
	}
	return result, nil
}

func fxAgreementToResult(ag *pb.FXAgreement) *FXAgreementResult {
	if ag == nil {
		return &FXAgreementResult{}
	}
	return &FXAgreementResult{
		TradeID:         ag.TradeId,
		Originator:      ag.Originator,
		CounterpartyB:   ag.CounterpartyB,
		SettlementAgent: ag.SettlementAgent,
		Custodian:       ag.Custodian,
		Beneficiary:     ag.Beneficiary,
		OriginAmount:    ag.OriginAmount,
		CounterAmount:   ag.CounterAmount,
		OriginCurrency:  ag.OriginCurrency,
		CounterCurrency: ag.CounterCurrency,
		Rate:            ag.Rate,
		ExpiryDate:      ag.ExpiryDate,
		SourceSpokeID:   ag.SourceSpokeId,
		DestSpokeID:     ag.DestSpokeId,
		SourceReceiver:  ag.SourceReceiver,
		DestReceiver:    ag.DestReceiver,
		State:           ag.State.String(),
		GroupID:         ag.GroupId,
		ContractAddress: ag.ContractAddress,
	}
}

func fxAgreementEventToResult(ev *pb.FXAgreementEvent) *FXAgreementEventResult {
	if ev == nil {
		return &FXAgreementEventResult{}
	}
	return &FXAgreementEventResult{
		ID:             ev.Id,
		TradeID:        ev.TradeId,
		FromState:      ev.FromState.String(),
		ToState:        ev.ToState.String(),
		Actor:          ev.Actor,
		OccurredAtUnix: ev.OccurredAtUnix,
		Notes:          ev.Notes,
		TxHash:         ev.TxHash,
		Source:         ev.Source.String(),
	}
}
