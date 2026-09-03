// SPDX-License-Identifier: Apache-2.0

package server

import (
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// callerGateway is the mTLS certificate CN the toolkit issues to the api-gateway.
// Every RPC below was verified to have exactly that one caller in this repository.
const callerGateway = "api-gateway"

// serverPolicy is the payment-orchestrator's per-method authorization policy
// (R2-H-8 follow-up item 2).
//
// This server is where value moves: it mints and burns tokens, locks and settles
// HTLCs, approves deposits, escrows and redeems, and drives Zeto transfers. Before
// this, holding any service-mesh certificate was enough to call all of it — the
// interceptor established WHO was calling and then authorized every method
// equally. The restriction below is the difference between "an authenticated peer"
// and "the one caller that legitimately performs this".
//
// The list is every value-moving or state-advancing method, restricted to the
// gateway. Reads (GetBalance, ListDeposits, SearchHTLC, …) keep the baseline
// policy, which still honours GRPC_AUTHZ_ALLOWED_CALLERS: they leak no authority,
// and tightening them would break the read paths other services legitimately use.
func serverPolicy() authz.Policy {
	return authz.RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
		// Token supply.
		pb.PaymentOrchestratorService_MintToken_FullMethodName,
		pb.PaymentOrchestratorService_BurnToken_FullMethodName,
		pb.PaymentOrchestratorService_TransferToken_FullMethodName,
		pb.PaymentOrchestratorService_InitiateZetoTransfer_FullMethodName,
		// Atomic settlement.
		pb.PaymentOrchestratorService_LockHTLC_FullMethodName,
		pb.PaymentOrchestratorService_LockHTLCWithHashLock_FullMethodName,
		pb.PaymentOrchestratorService_SettleHTLC_FullMethodName,
		pb.PaymentOrchestratorService_RefundHTLC_FullMethodName,
		// FX agreement lifecycle.
		pb.PaymentOrchestratorService_ProposeFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_AcceptFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_RejectFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_CancelFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_SettleFXAgreement_FullMethodName,
		// Reserve issuance and tokenisation: each approval moves fCeBM or tCeBM.
		pb.PaymentOrchestratorService_RegisterDeposit_FullMethodName,
		pb.PaymentOrchestratorService_ApproveDeposit_FullMethodName,
		pb.PaymentOrchestratorService_RejectDeposit_FullMethodName,
		pb.PaymentOrchestratorService_RequestFiatExchange_FullMethodName,
		pb.PaymentOrchestratorService_RequestEscrow_FullMethodName,
		pb.PaymentOrchestratorService_ApproveEscrow_FullMethodName,
		pb.PaymentOrchestratorService_RejectEscrow_FullMethodName,
		pb.PaymentOrchestratorService_RequestRedeem_FullMethodName,
		pb.PaymentOrchestratorService_ApproveRedeem_FullMethodName,
		pb.PaymentOrchestratorService_RejectRedeem_FullMethodName,
	)
}
