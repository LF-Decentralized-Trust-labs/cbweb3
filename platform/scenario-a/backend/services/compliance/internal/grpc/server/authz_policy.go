// SPDX-License-Identifier: Apache-2.0

package server

import (
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
)

// Service-mesh identities. These are the mTLS certificate CNs the toolkit issues,
// one per logical service, so the legitimate caller of an RPC is known at compile
// time (see the toolkit's gen-svc-tls step).
const (
	callerGateway = "api-gateway"
	callerAuth    = "auth"
)

// serverPolicy is the compliance service's per-method authorization policy
// (R2-H-8 follow-up item 2).
//
// Before this, an authenticated peer — any service holding a mesh certificate —
// could call every method, including the ones that admit a participant, freeze one,
// or halt the system. Authentication answered "who is calling"; nothing answered
// "may THIS caller call THIS method". These are the write methods where the answer
// is a short, known list.
//
// Each list was established by reading the call sites, not by guessing: a caller
// omitted here loses a working path the moment enforcement is switched on, so the
// lists are deliberately derived from what the code actually does today.
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
		// Governance actions reach compliance only through the gateway.
		RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
			compliancv1.ComplianceService_ApproveKYC_FullMethodName,
			compliancv1.ComplianceService_ToggleCircuitBreaker_FullMethodName,
			compliancv1.ComplianceService_UpdateSystemParameters_FullMethodName,
			compliancv1.ComplianceService_CreateTransferLimit_FullMethodName,
			compliancv1.ComplianceService_DeleteTransferLimit_FullMethodName,
		).
		// Freeze/suspend and CSR signing are driven by the gateway and by the auth
		// service's own onboarding path.
		WithRestriction([]string{callerGateway, callerAuth},
			compliancv1.ComplianceService_ManageParticipantStatus_FullMethodName,
			compliancv1.ComplianceService_SignParticipantCSR_FullMethodName,
		).
		// Certificate issuance is an auth-service step; the gateway never calls it.
		WithRestriction([]string{callerAuth},
			compliancv1.ComplianceService_IssueParticipantCertificate_FullMethodName,
		).
		// The participant record is written by auth during onboarding AND by the
		// gateway's governance route — see the adapter-rename note above.
		WithRestriction([]string{callerGateway, callerAuth},
			compliancv1.ComplianceService_UpsertParticipant_FullMethodName,
		).
		// Audit writes: restricted to the services that actually record events, so a
		// mesh peer cannot inject entries into the compliance trail. The actor on the
		// entry is still derived from the authenticated identity, never the payload.
		WithRestriction([]string{callerGateway, callerAuth},
			compliancv1.ComplianceService_CreateAuditLog_FullMethodName,
		)
}
