// SPDX-License-Identifier: Apache-2.0

package server

import (
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
)

// callerGateway is the mTLS certificate CN the toolkit issues to the api-gateway,
// the only caller of every method below in this repository.
const callerGateway = "api-gateway"

// serverPolicy is the payment-orchestrator's per-method authorization policy
// (R2-H-8 follow-up item 2).
//
// Every method listed moves value or advances settlement state: reserve issuance,
// tokenisation, redemption and the FX agreement lifecycle. Holding any
// service-mesh certificate used to be enough to call all of them. Reads keep the
// baseline policy, which still honours GRPC_AUTHZ_ALLOWED_CALLERS.
func serverPolicy() authz.Policy {
	return authz.RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
		// FX agreement lifecycle.
		pb.PaymentOrchestratorService_ProposeFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_AcceptFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_RejectFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_CancelFXAgreement_FullMethodName,
		pb.PaymentOrchestratorService_SettleFXAgreement_FullMethodName,
		// Reserve issuance, tokenisation and redemption: each approval moves fCeBM
		// or tCeBM on a spoke.
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
