// SPDX-License-Identifier: Apache-2.0

// This file registers API Gateway routes and attaches required dependencies.
package router

import (
	"os"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// Dependencies groups handlers and validators required by route registration.
type Dependencies struct {
	AuthHandler            *handlers.AuthHandler
	ComplianceHandler      *handlers.ComplianceHandler
	GovernanceHandler      *handlers.GovernanceHandler
	SupervisorHandler      *handlers.SupervisorHandler
	OversightHandler       *handlers.OversightHandler       // Investigation Module: AML/CFT disclosure requests
	TransferLimitHandler   *handlers.TransferLimitHandler   // Central Bank: Treasury transfer-limit CRUD (R1-10.1)
	PaymentHandler         *handlers.PaymentHandler         // Payment orchestrator: HTLC + token operations
	PaymentProxyHandler    *handlers.PaymentProxyHandler    // Commercial Bank: proxies escrow requests to CB
	OnboardingHandler      *handlers.OnboardingHandler      // Central Bank: processes onboarding locally
	OnboardingProxyHandler *handlers.OnboardingProxyHandler // Commercial Bank: proxies onboarding to CB
	IdentityHandler        *handlers.IdentityHandler        // Paladin identity choices for the FX agreement form
	StatementHandler       *handlers.StatementHandler       // Commercial Bank: consolidated fCeBM/tCeBM statement (extrato)
	AuthProvider           interfaces.IAuthProvider
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	app.Get("/openapi.yaml", handlers.OpenAPIYAML)
	app.Get("/docs", handlers.SwaggerUI)
	// Vendored, embedded Swagger UI assets — served locally so /docs works offline (no CDN).
	app.Get("/docs/swagger-ui/:asset", handlers.SwaggerUIAsset)
	app.Get("/healthz", handlers.Health)

	// --- Auth ---
	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/login", deps.AuthHandler.Login)
	authGroup.Post("/refresh", deps.AuthHandler.Refresh)
	authGroup.Post("/logout", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.Logout)
	// PKI login step 2: submit signed nonce + X.509 certificate
	authGroup.Post("/wallet/bind", deps.AuthHandler.WalletBind)
	// Client secret rotation (requires valid access_token cookie)
	authGroup.Post("/client-secret/change", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.ChangeClientSecret)
	// Self-profile: returns token claims for the authenticated caller
	authGroup.Get("/me", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.Me)

	// --- Onboarding (3-phase PKI + Blockchain) ---
	if deps.OnboardingProxyHandler != nil {
		// Commercial Bank: protected proxy routes forwarding to the Central Bank.
		g := app.Group("/api/v1/onboarding", middleware.RequireCookieAuth(deps.AuthProvider))
		g.Post("/initiate", deps.OnboardingProxyHandler.InitiateCredentialRequest)
		g.Get("/status/:requestId", deps.OnboardingProxyHandler.GetOnboardingStatus)
		// Recovers status without the original request_id (e.g. after a page reload).
		g.Get("/my-status", deps.OnboardingProxyHandler.GetMyOnboardingStatus)
		g.Post("/complete", deps.OnboardingProxyHandler.CompleteOnboarding)

		// PKI re-login: authenticates the commercial bank against the CB.
		authGroup.Post("/pki-login", middleware.RequireCookieAuth(deps.AuthProvider), deps.OnboardingProxyHandler.PKILogin)
	} else if deps.OnboardingHandler != nil {
		// Central Bank: public endpoints that process onboarding requests.
		g := app.Group("/api/v1/onboarding")
		g.Post("/credential-request", deps.OnboardingHandler.SubmitCredentialRequest)
		g.Get("/status/:requestId", deps.OnboardingHandler.GetOnboardingStatus)
		// Lookup by bank_code for frontends that lost the original request_id.
		g.Get("/my-status", deps.OnboardingHandler.GetMyOnboardingStatus)
		g.Post("/complete", deps.OnboardingHandler.CompleteOnboarding)
	}

	// --- Compliance (KYC status, AML gate, onboarding, provisioning) ---
	complianceGroup := app.Group("/api/v1/compliance", middleware.RequireCookieAuth(deps.AuthProvider))
	complianceGroup.Get("/kyc/status/:subject", deps.ComplianceHandler.GetKYCStatus)
	complianceGroup.Post("/aml/screen", deps.ComplianceHandler.AMLScreen)
	if deps.SupervisorHandler != nil {
		complianceGroup.Get("/audit/logs", middleware.RequireSupervisorRole(), deps.SupervisorHandler.GetAuditLogs)
		complianceGroup.Post("/decrypt-transaction", middleware.RequireSupervisorRole(), deps.SupervisorHandler.DecryptTransaction)

		// Read-only participants list for supervisor (same handler, no write access).
		complianceGroup.Get("/participants/summary", middleware.RequireSupervisorRole(), deps.ComplianceHandler.ListParticipants)
	}

	// --- Investigation Module (AML/CFT Disclosure Requests — FR-034/FR-035/FR-036) ---
	if deps.OversightHandler != nil {
		oversightGroup := app.Group("/api/v1/oversight", middleware.RequireCookieAuth(deps.AuthProvider), middleware.RequireSupervisorRole())
		oversightGroup.Post("/disclosure-request", deps.OversightHandler.OpenDisclosure)
		oversightGroup.Post("/disclosure-sign", deps.OversightHandler.SignDisclosure)
		oversightGroup.Get("/disclosure-status/:requestID", deps.OversightHandler.GetDisclosureStatus)
	}

	centralBankRoutes := complianceGroup.Group("", middleware.RequireRole(domain.RoleGovernance))
	centralBankRoutes.Get("/participants", deps.ComplianceHandler.ListParticipants)
	centralBankRoutes.Post("/approve-kyc", deps.GovernanceHandler.ApproveKYC)
	centralBankRoutes.Post("/participants/provision", deps.ComplianceHandler.ProvisionParticipant)
	centralBankRoutes.Post("/accounts/freeze", deps.ComplianceHandler.FreezeAccount)
	centralBankRoutes.Post("/accounts/unfreeze", deps.ComplianceHandler.UnfreezeAccount)
	centralBankRoutes.Post("/register", deps.ComplianceHandler.RegisterParticipant)

	// --- Governance Portal (Central Bank only — ROLE_GOVERNANCE) ---
	// Only registered when running as a central bank gateway (no proxy handler).
	// Scope: KYC/onboarding/registry, account control (freeze/unfreeze),
	// circuit breaker, system parameters, and audit logs. Operational actions
	// (mint/burn, deposit/escrow/redeem approvals) live under ROLE_TREASURY.
	if deps.PaymentProxyHandler == nil && deps.GovernanceHandler != nil {
		govGroup := app.Group("/api/v1/governance",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireRole(domain.RoleGovernance),
		)
		govGroup.Post("/approve-kyc", deps.GovernanceHandler.ApproveKYC)

		govGroup.Get("/registry", deps.GovernanceHandler.GetRegistry)
		govGroup.Post("/registry/csr", deps.GovernanceHandler.SubmitCSR)
		govGroup.Post("/participants", deps.GovernanceHandler.RegisterParticipant)

		govGroup.Get("/accounts", deps.GovernanceHandler.GetAccounts)
		govGroup.Post("/accounts/freeze", deps.GovernanceHandler.FreezeAccount)
		govGroup.Post("/accounts/unfreeze", deps.GovernanceHandler.UnfreezeAccount)

		govGroup.Get("/circuit-breaker/status", deps.GovernanceHandler.GetCircuitBreakerStatus)
		govGroup.Post("/circuit-breaker/toggle", deps.GovernanceHandler.ToggleCircuitBreaker)

		govGroup.Get("/parameters", deps.GovernanceHandler.GetParameters)
		govGroup.Put("/parameters", deps.GovernanceHandler.UpdateParameters)

		govGroup.Get("/audit/logs", deps.GovernanceHandler.GetAuditLogs)

		govGroup.Get("/users", deps.GovernanceHandler.ListUsers)
		govGroup.Get("/users/:userId", deps.GovernanceHandler.GetUser)
	}

	// --- Treasury: Transfer Limits (Central Bank only — ROLE_TREASURY, R1-10.1) ---
	if deps.PaymentProxyHandler == nil && deps.TransferLimitHandler != nil {
		tlGroup := app.Group("/api/v1/treasury/transfer-limits",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireRole(domain.RoleTreasury),
		)
		tlGroup.Post("", deps.TransferLimitHandler.CreateTransferLimit)
		tlGroup.Get("", deps.TransferLimitHandler.ListTransferLimits)
		tlGroup.Delete("/:id", deps.TransferLimitHandler.DeleteTransferLimit)
	}

	// --- Paladin identities (FX agreement party choices) ---
	if deps.IdentityHandler != nil {
		identityGroup := app.Group("/api/v1/identities", middleware.RequireCookieAuth(deps.AuthProvider))
		identityGroup.Get("", deps.IdentityHandler.ListIdentities)
	}

	// --- Statement / Extrato (commercial bank only) ---
	if deps.StatementHandler != nil {
		statementGroup := app.Group("/api/v1/statement", middleware.RequireCookieAuth(deps.AuthProvider))
		statementGroup.Get("", deps.StatementHandler.GetStatement)
	}

	// --- Payment Orchestrator (HTLC + Token) ---
	if deps.PaymentHandler != nil {
		payGroup := app.Group("/api/v1", middleware.RequireCookieAuth(deps.AuthProvider))

		htlcGroup := payGroup.Group("/htlc")
		htlcGroup.Post("/lock", deps.PaymentHandler.LockHTLC)
		htlcGroup.Post("/lock-with-hash", deps.PaymentHandler.LockHTLCWithHashLock)
		htlcGroup.Post("/settle", deps.PaymentHandler.SettleHTLC)
		htlcGroup.Post("/refund", deps.PaymentHandler.RefundHTLC)
		htlcGroup.Get("/status/:contractId", deps.PaymentHandler.GetHTLCStatus)
		htlcGroup.Get("/search", deps.PaymentHandler.SearchHTLC)

		tokenGroup := payGroup.Group("/token")
		tokenGroup.Post("/transfer", deps.PaymentHandler.TransferToken)
		tokenGroup.Get("/balance", deps.PaymentHandler.GetBalance)
		tokenGroup.Get("/fiat-balance", deps.PaymentHandler.GetFiatBalance)
		// Sovereign mint is a treasury operation; restrict at the central bank gateway only.
		if deps.PaymentProxyHandler == nil {
			tokenGroup.Post("/mint", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.MintToken)
		} else {
			tokenGroup.Post("/mint", deps.PaymentHandler.MintToken)
		}

		// FX Agreement routes
		fxGroup := payGroup.Group("/payments/fx/agreements")
		fxGroup.Post("", deps.PaymentHandler.ProposeFXAgreement)
		fxGroup.Post("/:tradeId/accept", deps.PaymentHandler.AcceptFXAgreement)
		fxGroup.Post("/:tradeId/reject", deps.PaymentHandler.RejectFXAgreement)
		fxGroup.Post("/:tradeId/cancel", deps.PaymentHandler.CancelFXAgreement)
		fxGroup.Post("/:tradeId/settle", deps.PaymentHandler.SettleFXAgreement)
		fxGroup.Get("/:tradeId", deps.PaymentHandler.GetFXAgreement)
		fxGroup.Get("/:tradeId/audit", deps.PaymentHandler.ListFXAgreementEvents)
		fxGroup.Get("", deps.PaymentHandler.ListFXAgreements)

		// Internal FX route for the Cacti relay — protected by X-Relay-Auth service-to-service secret.
		// INTERNAL_RELAY_AUTH_SECRET must be set in the API Gateway environment.
		relayAuthSecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
		intFX := app.Group("/internal/v1/payments/fx/agreements", middleware.RequireRelayAuth(relayAuthSecret))
		intFX.Get("", deps.PaymentHandler.ListFXAgreements)

		// --- Escrow: Deposit / Escrow / Redeem (Central Bank) ---
		if deps.PaymentProxyHandler == nil {
			// Sovereign token burn — treasury-only, central bank gateway only.
			tokenGroup.Post("/burn", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.BurnToken)

			// Internal routes for proxy-receiving — protected by X-Relay-Auth shared secret.
			intDeposits := app.Group("/internal/v1/payments/deposits", middleware.RequireRelayAuth(relayAuthSecret))
			intDeposits.Post("", deps.PaymentHandler.RegisterDeposit)
			intDeposits.Get("", deps.PaymentHandler.ListDeposits)

			intEscrows := app.Group("/internal/v1/payments/escrows", middleware.RequireRelayAuth(relayAuthSecret))
			intEscrows.Post("", deps.PaymentHandler.RequestEscrow)
			intEscrows.Get("", deps.PaymentHandler.ListEscrows)

			intRedeems := app.Group("/internal/v1/payments/redeems", middleware.RequireRelayAuth(relayAuthSecret))
			intRedeems.Post("", deps.PaymentHandler.RequestRedeem)
			intRedeems.Get("", deps.PaymentHandler.ListRedeems)

			// Operational approvals — ROLE_TREASURY (treasury portal).
			depositGroup := payGroup.Group("/payments/deposits")
			depositGroup.Post("/approve", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.ApproveDeposit)
			depositGroup.Post("/reject", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.RejectDeposit)
			depositGroup.Post("/fiat-exchange", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.RequestFiatExchange)
			depositGroup.Get("", deps.PaymentHandler.ListDeposits)

			escrowGroup := payGroup.Group("/payments/escrows")
			escrowGroup.Post("/approve", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.ApproveEscrow)
			escrowGroup.Post("/reject", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.RejectEscrow)
			escrowGroup.Get("", deps.PaymentHandler.ListEscrows)

			redeemGroup := payGroup.Group("/payments/redeems")
			redeemGroup.Post("/approve", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.ApproveRedeem)
			redeemGroup.Post("/reject", middleware.RequireRole(domain.RoleTreasury), deps.PaymentHandler.RejectRedeem)
			redeemGroup.Get("", deps.PaymentHandler.ListRedeems)
		}
	}

	// --- Payment Proxy (Commercial Bank → Central Bank) ---
	if deps.PaymentProxyHandler != nil {
		proxyGroup := app.Group("/api/v1/payments", middleware.RequireCookieAuth(deps.AuthProvider))
		proxyGroup.Post("/deposits", deps.PaymentProxyHandler.RegisterDeposit)
		proxyGroup.Get("/deposits", deps.PaymentProxyHandler.ListDeposits)
		proxyGroup.Post("/escrows", deps.PaymentProxyHandler.RequestEscrow)
		proxyGroup.Get("/escrows", deps.PaymentProxyHandler.ListEscrows)
		proxyGroup.Post("/redeems", deps.PaymentProxyHandler.RequestRedeem)
		proxyGroup.Get("/redeems", deps.PaymentProxyHandler.ListRedeems)
	}
}
