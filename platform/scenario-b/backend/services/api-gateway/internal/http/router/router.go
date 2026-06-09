// Package router registers API Gateway routes and attaches required dependencies.
// Scenario B v2 routes are registered by the v2 sub-package (T045/T070/T091).
package router

import (
	"os"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	v2router "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/router/v2"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// Dependencies groups handlers and validators required by route registration.
type Dependencies struct {
	AuthHandler            *handlers.AuthHandler
	ComplianceHandler      *handlers.ComplianceHandler
	GovernanceHandler      *handlers.GovernanceHandler
	OnboardingHandler      *handlers.OnboardingHandler
	OnboardingProxyHandler *handlers.OnboardingProxyHandler
	AuthProvider           interfaces.IAuthProvider
	// PaymentProxyHandler proxies /api/v1/payments/* to the Central Bank gateway.
	// Only wired when CENTRAL_BANK_API_URL is set (commercial bank gateways).
	PaymentProxyHandler *handlers.PaymentProxyHandler
	// PaymentHandler serves /api/v1/token/* endpoints via gRPC to the payment-orchestrator.
	// Only wired when PAYMENT_GRPC_ADDR is set.
	PaymentHandler *handlers.PaymentHandler
	V2Deps         v2router.Dependencies
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
	authGroup.Post("/wallet/bind", deps.AuthHandler.WalletBind)
	authGroup.Post("/client-secret/change", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.ChangeClientSecret)
	authGroup.Get("/me", middleware.RequireCookieAuth(deps.AuthProvider), deps.AuthHandler.Me)

	// --- Onboarding (3-phase PKI + Blockchain) ---
	if deps.OnboardingProxyHandler != nil {
		g := app.Group("/api/v1/onboarding", middleware.RequireCookieAuth(deps.AuthProvider))
		g.Post("/initiate", deps.OnboardingProxyHandler.InitiateCredentialRequest)
		g.Get("/status/:requestId", deps.OnboardingProxyHandler.GetOnboardingStatus)
		g.Get("/my-status", deps.OnboardingProxyHandler.GetMyOnboardingStatus)
		g.Post("/complete", deps.OnboardingProxyHandler.CompleteOnboarding)

		authGroup.Post("/pki-login", middleware.RequireCookieAuth(deps.AuthProvider), deps.OnboardingProxyHandler.PKILogin)
	} else if deps.OnboardingHandler != nil {
		g := app.Group("/api/v1/onboarding")
		g.Post("/credential-request", deps.OnboardingHandler.SubmitCredentialRequest)
		g.Get("/status/:requestId", deps.OnboardingHandler.GetOnboardingStatus)
		g.Get("/my-status", deps.OnboardingHandler.GetMyOnboardingStatus)
		g.Post("/complete", deps.OnboardingHandler.CompleteOnboarding)
	}

	// --- Compliance (KYC status, AML gate — infrastructure reused from Scenario A) ---
	complianceGroup := app.Group("/api/v1/compliance", middleware.RequireCookieAuth(deps.AuthProvider))
	complianceGroup.Get("/kyc/status/:subject", deps.ComplianceHandler.GetKYCStatus)
	complianceGroup.Post("/aml/screen", deps.ComplianceHandler.AMLScreen)
	complianceGroup.Get("/participants", middleware.RequireRole("ROLE_GOVERNANCE"), deps.ComplianceHandler.ListParticipants)
	complianceGroup.Post("/approve-kyc", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.ApproveKYC)

	// --- Governance Portal ---
	govGroup := app.Group("/api/v1/governance",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireRole("ROLE_GOVERNANCE"),
	)
	govGroup.Post("/participants", deps.GovernanceHandler.RegisterParticipant)
	govGroup.Get("/registry", deps.GovernanceHandler.GetRegistry)
	govGroup.Post("/registry/csr", deps.GovernanceHandler.SubmitCSR)
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

	// --- Payment Proxy (commercial bank → Central Bank) ---
	// Routes: POST/GET /api/v1/payments/{deposits,redeems}
	// Only active when CENTRAL_BANK_API_URL is configured (commercial bank gateways).
	// The proxy injects requester_besu_address automatically from gateway config —
	// the frontend only needs to send {amount} for most calls.
	if deps.PaymentProxyHandler != nil {
		payments := app.Group("/api/v1/payments", middleware.RequireCookieAuth(deps.AuthProvider))
		payments.Post("/deposits/exchange", deps.PaymentProxyHandler.RequestFiatExchange)
		payments.Post("/deposits", deps.PaymentProxyHandler.RegisterDeposit)
		payments.Get("/deposits", deps.PaymentProxyHandler.ListDeposits)
		payments.Post("/escrows", deps.PaymentProxyHandler.RequestEscrow)
		payments.Get("/escrows", deps.PaymentProxyHandler.ListEscrows)
		payments.Post("/redeems", deps.PaymentProxyHandler.RequestRedeem)
		payments.Get("/redeems", deps.PaymentProxyHandler.ListRedeems)
	}

	// --- Payment Handler — Central Bank gateway routes (direct gRPC, no proxy) ---
	// Deposits and redeems with approve/reject — only on CB gateways
	// (PAYMENT_GRPC_ADDR set, CENTRAL_BANK_API_URL not set → PaymentProxyHandler == nil).
	if deps.PaymentHandler != nil && deps.PaymentProxyHandler == nil {
		payments := app.Group("/api/v1/payments", middleware.RequireCookieAuth(deps.AuthProvider))
		payments.Post("/deposits/approve", deps.PaymentHandler.ApproveDeposit)
		payments.Post("/deposits/reject", deps.PaymentHandler.RejectDeposit)
		payments.Post("/deposits/exchange", deps.PaymentHandler.RequestFiatExchange)
		payments.Get("/deposits", deps.PaymentHandler.ListDeposits)
		payments.Get("/escrows", deps.PaymentHandler.ListEscrows)
		payments.Post("/escrows/approve", deps.PaymentHandler.ApproveEscrow)
		payments.Post("/escrows/reject", deps.PaymentHandler.RejectEscrow)
		payments.Post("/redeems/approve", deps.PaymentHandler.ApproveRedeem)
		payments.Post("/redeems/reject", deps.PaymentHandler.RejectRedeem)
		payments.Get("/redeems", deps.PaymentHandler.ListRedeems)
	}

	// --- Token balance (tCeBM via payment-orchestrator gRPC) ---
	if deps.PaymentHandler != nil {
		token := app.Group("/api/v1/token", middleware.RequireCookieAuth(deps.AuthProvider))
		token.Get("/balance", deps.PaymentHandler.GetBalance)
		token.Get("/fiat-balance", deps.PaymentHandler.GetFiatBalance)
	}

	// --- Internal Relay Endpoints (for commercial bank proxy) ---
	// Protected by X-Relay-Auth header, used by PaymentProxyHandler from commercial banks.
	// Only wired on Central Bank gateways (PaymentHandler exists, PaymentProxyHandler doesn't).
	if deps.PaymentHandler != nil && deps.PaymentProxyHandler == nil {
		relaySecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
		internal := app.Group("/internal/v1", middleware.RequireRelayAuth(relaySecret))

		internalPayments := internal.Group("/payments")
		internalPayments.Post("/deposits/exchange", deps.PaymentHandler.RequestFiatExchange)
		internalPayments.Post("/deposits", deps.PaymentHandler.RegisterDeposit)
		internalPayments.Get("/deposits", deps.PaymentHandler.ListDeposits)
		internalPayments.Post("/escrows", deps.PaymentHandler.RequestEscrow)
		internalPayments.Get("/escrows", deps.PaymentHandler.ListEscrows)
		internalPayments.Post("/redeems", deps.PaymentHandler.RequestRedeem)
		internalPayments.Get("/redeems", deps.PaymentHandler.ListRedeems)
	}

	// --- Scenario B API v2 ---
	v2router.Register(app, deps.V2Deps)
}
