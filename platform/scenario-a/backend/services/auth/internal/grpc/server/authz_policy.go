// SPDX-License-Identifier: Apache-2.0

package server

import (
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
)

// Service-mesh identities: the mTLS certificate CNs the toolkit issues per service.
const (
	callerGateway    = "api-gateway"
	callerCompliance = "compliance"
)

// serverPolicy is the auth service's per-method authorization policy (R2-H-8
// follow-up item 2).
//
// The methods restricted here create identities: they provision Keycloak users,
// derive wallets and register participants on-chain. An authenticated peer holding
// any mesh certificate could previously drive all of it. Login, token validation
// and the read methods keep the baseline policy — they are what every service and
// the gateway legitimately call, and the login path is how an unauthenticated user
// becomes authenticated in the first place.
func serverPolicy() authz.Policy {
	return authz.
		RestrictMethods(authz.DefaultPolicyFromEnv(), []string{callerGateway},
			authv1.AuthService_OnboardParticipant_FullMethodName,
			authv1.AuthService_ProvisionParticipant_FullMethodName,
			authv1.AuthService_CompleteOnboarding_FullMethodName,
			authv1.AuthService_ChangeClientSecret_FullMethodName,
			authv1.AuthService_SignTransaction_FullMethodName,
		).
		// Registration is driven by the gateway and by compliance's KYC approval,
		// which registers the approved bank's wallet.
		WithRestriction([]string{callerGateway, callerCompliance},
			authv1.AuthService_RegisterParticipant_FullMethodName,
		)
}
