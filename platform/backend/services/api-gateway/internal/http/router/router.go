// This file registers API Gateway routes and attaches required dependencies.
package router

import (
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
	PaymentHandler         *handlers.PaymentHandler         // Payment orchestrator: HTLC + token operations
	PaymentProxyHandler    *handlers.PaymentProxyHandler    // Commercial Bank: proxies escrow requests to CB
	OnboardingHandler      *handlers.OnboardingHandler      // Central Bank: processes onboarding locally
	OnboardingProxyHandler *handlers.OnboardingProxyHandler // Commercial Bank: proxies onboarding to CB
	AuthProvider           interfaces.IAuthProvider
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	app.Get("/openapi.yaml", handlers.OpenAPIYAML)
	app.Get("/docs", handlers.SwaggerUI)
	app.Get("/healthz", handlers.Health)

	// --- Auth ---
	authGroup := app.Group("/api/v1/auth")
	authGroup.Post("/login", deps.AuthHandler.Login)
	authGroup.Post("/refresh", deps.AuthHandler.Refresh)
	authGroup.Post("/logout", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.Logout)
	// PKI login step 2: submit signed nonce + X.509 certificate
	authGroup.Post("/wallet/bind", deps.AuthHandler.WalletBind)
	// MVP-only: signs a PKI nonce using a .pem file from PKI_DIR; remove before production
	authGroup.Post("/resolve-challenge", deps.AuthHandler.ResolveChallenger)
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

	centralBankRoutes := complianceGroup.Group("", middleware.RequireRole(domain.RoleGovernance))
	centralBankRoutes.Get("/participants", deps.ComplianceHandler.ListParticipants)
	centralBankRoutes.Post("/approve-kyc", deps.GovernanceHandler.ApproveKYC)
	centralBankRoutes.Post("/participants/provision", deps.ComplianceHandler.ProvisionParticipant)
	centralBankRoutes.Post("/accounts/freeze", deps.ComplianceHandler.FreezeAccount)
	centralBankRoutes.Post("/accounts/unfreeze", deps.ComplianceHandler.UnfreezeAccount)
	centralBankRoutes.Post("/register", deps.ComplianceHandler.RegisterParticipant)

	// --- Governance Portal (PKI / ROLE_GOVERNANCE only) ---
	govGroup := app.Group("/api/v1/governance",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireRole(domain.RoleGovernance),
	)
	// Participant registration and registry
	govGroup.Post("/participants", deps.GovernanceHandler.RegisterParticipant)
	govGroup.Get("/registry", deps.GovernanceHandler.GetRegistry)
	govGroup.Post("/registry/csr", deps.GovernanceHandler.SubmitCSR)

	// Account management
	govGroup.Get("/accounts", deps.GovernanceHandler.GetAccounts)
	govGroup.Post("/accounts/freeze", deps.GovernanceHandler.FreezeAccount)
	govGroup.Post("/accounts/unfreeze", deps.GovernanceHandler.UnfreezeAccount)

	// Circuit breaker
	govGroup.Get("/circuit-breaker/status", deps.GovernanceHandler.GetCircuitBreakerStatus)
	govGroup.Post("/circuit-breaker/toggle", deps.GovernanceHandler.ToggleCircuitBreaker)

	// Global parameters
	govGroup.Get("/parameters", deps.GovernanceHandler.GetParameters)
	govGroup.Put("/parameters", deps.GovernanceHandler.UpdateParameters)

	// Audit logs
	govGroup.Get("/audit/logs", deps.GovernanceHandler.GetAuditLogs)

	// User management (list and get registered participants)
	govGroup.Get("/users", deps.GovernanceHandler.ListUsers)
	govGroup.Get("/users/:userId", deps.GovernanceHandler.GetUser)

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
		tokenGroup.Post("/mint", deps.PaymentHandler.MintToken)
		tokenGroup.Post("/transfer", deps.PaymentHandler.TransferToken)
		tokenGroup.Get("/balance", deps.PaymentHandler.GetBalance)
		tokenGroup.Get("/fiat-balance", deps.PaymentHandler.GetFiatBalance)

		// FX Agreement routes
		fxGroup := payGroup.Group("/payments/fx/agreements")
		fxGroup.Post("", deps.PaymentHandler.ProposeFXAgreement)
		fxGroup.Post("/:tradeId/accept", deps.PaymentHandler.AcceptFXAgreement)
		fxGroup.Post("/:tradeId/reject", deps.PaymentHandler.RejectFXAgreement)
		fxGroup.Post("/:tradeId/cancel", deps.PaymentHandler.CancelFXAgreement)
		fxGroup.Post("/:tradeId/settle", deps.PaymentHandler.SettleFXAgreement)
		fxGroup.Get("/:tradeId", deps.PaymentHandler.GetFXAgreement)
		fxGroup.Get("", deps.PaymentHandler.ListFXAgreements)

		// Internal FX route for the Cacti relay (no auth — only reachable within Docker network).
		intFX := app.Group("/internal/v1/payments/fx/agreements")
		intFX.Get("", deps.PaymentHandler.ListFXAgreements)

		// --- Escrow: Deposit / Escrow / Redeem (Central Bank) ---
		if deps.PaymentProxyHandler == nil {
			// Internal routes for proxy-receiving (no auth — only reachable within Docker network).
			intDeposits := app.Group("/internal/v1/payments/deposits")
			intDeposits.Post("", deps.PaymentHandler.RegisterDeposit)
			intDeposits.Get("", deps.PaymentHandler.ListDeposits)

			intEscrows := app.Group("/internal/v1/payments/escrows")
			intEscrows.Post("", deps.PaymentHandler.RequestEscrow)
			intEscrows.Get("", deps.PaymentHandler.ListEscrows)

			intRedeems := app.Group("/internal/v1/payments/redeems")
			intRedeems.Post("", deps.PaymentHandler.RequestRedeem)
			intRedeems.Get("", deps.PaymentHandler.ListRedeems)

			// Governance routes (requires cookie auth + ROLE_GOVERNANCE).
			depositGroup := payGroup.Group("/payments/deposits")
			depositGroup.Post("/approve", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.ApproveDeposit)
			depositGroup.Post("/reject", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.RejectDeposit)
			depositGroup.Post("/fiat-exchange", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.RequestFiatExchange)
			depositGroup.Get("", deps.PaymentHandler.ListDeposits)

			escrowGroup := payGroup.Group("/payments/escrows")
			escrowGroup.Post("/approve", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.ApproveEscrow)
			escrowGroup.Post("/reject", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.RejectEscrow)
			escrowGroup.Get("", deps.PaymentHandler.ListEscrows)

			redeemGroup := payGroup.Group("/payments/redeems")
			redeemGroup.Post("/approve", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.ApproveRedeem)
			redeemGroup.Post("/reject", middleware.RequireRole(domain.RoleGovernance), deps.PaymentHandler.RejectRedeem)
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
