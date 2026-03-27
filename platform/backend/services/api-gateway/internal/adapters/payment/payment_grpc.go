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
	ContractID  string `json:"contract_id"`
	Sender      string `json:"sender"`
	Receiver    string `json:"receiver"`
	HashLock    string `json:"hash_lock"`
	TimeLock    uint64 `json:"time_lock"`
	Secret      string `json:"secret,omitempty"`
	ZetoLockRef string `json:"zeto_lock_ref"`
	State       string `json:"state"`
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

func (a *GRPCAdapter) GetBalance(ctx context.Context, identity string) (*BalanceResult, error) {
	resp, err := a.cc.GetBalance(ctx, &pb.GetBalanceRequest{Identity: identity})
	if err != nil {
		return nil, err
	}
	return &BalanceResult{Balance: resp.Balance}, nil
}

func lockToStatus(l *pb.HTLCLock) *HTLCStatus {
	if l == nil {
		return &HTLCStatus{}
	}
	return &HTLCStatus{
		ContractID:  l.ContractId,
		Sender:      l.Sender,
		Receiver:    l.Receiver,
		HashLock:    l.HashLock,
		TimeLock:    l.TimeLock,
		Secret:      l.Secret,
		ZetoLockRef: l.ZetoLockRef,
		State:       l.State.String(),
	}
}
