// SPDX-License-Identifier: Apache-2.0

package server

import (
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
)

// callerGateway is the mTLS certificate CN the toolkit issues to the api-gateway.
const callerGateway = "api-gateway"

// serverPolicy is the auth service's per-method authorization policy (R2-H-8
// follow-up item 2).
//
// The restricted methods create identities: Keycloak users, derived wallets and
// on-chain participant registrations. Login, token validation and reads keep the
// baseline policy — the login path is how a caller becomes authenticated at all,
// and every service legitimately validates tokens.
func serverPolicy() authz.Policy {
	return authz.RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
		authv1.AuthService_OnboardParticipant_FullMethodName,
		authv1.AuthService_RegisterParticipant_FullMethodName,
		authv1.AuthService_ProvisionParticipant_FullMethodName,
		authv1.AuthService_CompleteOnboarding_FullMethodName,
		authv1.AuthService_ChangeClientSecret_FullMethodName,
		authv1.AuthService_SignTransaction_FullMethodName,
	)
}
