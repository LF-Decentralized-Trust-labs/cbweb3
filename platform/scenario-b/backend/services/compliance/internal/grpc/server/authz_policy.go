// SPDX-License-Identifier: Apache-2.0

package server

import (
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
)

// Service-mesh identities: the mTLS certificate CNs the toolkit issues per service.
const (
	callerGateway = "api-gateway"
	callerAuth    = "auth"
)

// serverPolicy is the compliance service's per-method authorization policy
// (R2-H-8 follow-up item 2).
//
// Authentication established who was calling; nothing established whether that
// caller may call a given method, so any peer holding a mesh certificate could
// admit a participant, freeze one, halt the system, or register a currency and a
// corridor on the hub. Each list below is the set of callers the code actually has
// today — read from the call sites, because a caller omitted here loses a working
// path the moment enforcement is switched on.
//
// Reads keep the baseline policy, which still honours GRPC_AUTHZ_ALLOWED_CALLERS.//
// One trap, named because it already caught this file once: a caller list derived by
// grepping for the gRPC method name is WRONG when the gateway reaches an RPC through
// an adapter method with a different name. UpsertParticipant is exactly that —
// POST /governance/participants → GovernanceHandler.RegisterParticipant →
// GRPCAdapter.RegisterParticipant → compliance.UpsertParticipant. Verify a caller set
// by searching for the *gRPC* call (`.UpsertParticipant(ctx`) across every service,
// not for the RPC's name as a Go method.
func serverPolicy() authz.Policy {
	return authz.
		RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
			compliancv1.ComplianceService_ApproveKYC_FullMethodName,
			compliancv1.ComplianceService_ToggleCircuitBreaker_FullMethodName,
			compliancv1.ComplianceService_UpdateSystemParameters_FullMethodName,
			// Sovereign on-chain governance: issuing a currency and opening a
			// corridor are central-bank acts, driven through the gateway.
			compliancv1.ComplianceService_RegisterCurrencyOnChain_FullMethodName,
			compliancv1.ComplianceService_RegisterPairOnChain_FullMethodName,
			compliancv1.ComplianceService_RegisterParticipantOnChain_FullMethodName,
		).
		WithRestriction([]string{callerGateway, callerAuth},
			compliancv1.ComplianceService_ManageParticipantStatus_FullMethodName,
			compliancv1.ComplianceService_SignParticipantCSR_FullMethodName,
		).
		// Certificate issuance and audit writes are auth-service steps here, so a mesh
		// peer cannot inject entries into the compliance trail. The actor on an entry is
		// still derived from the authenticated identity, never from the payload.
		WithRestriction([]string{callerAuth},
			compliancv1.ComplianceService_IssueParticipantCertificate_FullMethodName,
			compliancv1.ComplianceService_CreateAuditLog_FullMethodName,
		).
		// The participant record is written by auth during onboarding AND by the
		// gateway's governance route — see the adapter-rename note above.
		WithRestriction([]string{callerGateway, callerAuth},
			compliancv1.ComplianceService_UpsertParticipant_FullMethodName,
		)
}
