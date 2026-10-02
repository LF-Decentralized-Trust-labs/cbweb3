// SPDX-License-Identifier: Apache-2.0

// This file registers API Gateway routes and attaches required dependencies.
package router

import (
	"strings"

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
	// CSRFSecret keys the HMAC binding a CSRF token to its session. When empty the
	// guard still runs and refuses every mutating request: a gateway that silently
	// stopped enforcing CSRF because a value was missing is the failure this control
	// exists to prevent.
	CSRFSecret []byte
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	// CSRF is mounted app-wide, BEFORE any route is registered, so a route added
	// later is protected without anyone remembering to protect it. Scenario A had 54
	// mutating routes across 11 cookie-authenticated groups and no CSRF at all;
	// per-group wiring is what let that happen in the sibling scenario too.
	//
	// An independent copy of the guard, not a shared module: Constitution Principle I
	// forbids reaching across scenarios, and the R2-H-13 plan justified per-scenario
	// copies for exactly this control.
	app.Use(middleware.CSRF(middleware.CSRFConfig{
		Secret:    deps.CSRFSecret,
		SessionID: func(c *fiber.Ctx) string { return c.Cookies("access_token") },
		Exempt:    csrfExempt,
	}))

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

	// The sub-group guard is the UNION of the roles used inside it (spec 042). Fiber
	// group middleware is PREFIX-scoped, and Group("") mounts at the parent prefix, so
	// a route-level RequireRole runs *in addition* to this one: a narrower group guard
	// would 403 an Admission caller before its own route guard ran.
	//
	// CONSEQUENCE: this group no longer authorizes anything by itself. EVERY route below
	// must carry an explicit guard; one added without a guard is reachable by BOTH
	// profiles (INV-5 in contracts/authorization-matrix.md).
	centralBankRoutes := complianceGroup.Group("", middleware.RequireRole(domain.RoleGovernance, domain.RoleAdmission))
	// Shared read view / Admission-exclusive approval.
	centralBankRoutes.Get("/participants", middleware.RequireRole(domain.RoleGovernance, domain.RoleAdmission), deps.ComplianceHandler.ListParticipants)
	centralBankRoutes.Post("/approve-kyc", middleware.RequireRole(domain.RoleAdmission), deps.GovernanceHandler.ApproveKYC)
	// Governance-retained: provision can set FROZEN (a freeze lever, not onboarding);
	// freeze/unfreeze are value controls; /register provisions CENTRAL-BANK operator
	// accounts (its IsAdminRole allowlist admits TREASURY/NOC/SUPERVISOR) and performs
	// an inline CB-signed on-chain register+verify via auth OnboardParticipant, so it
	// must stay off the Admission surface (FR-003b / FR-015a — decision T029a).
	centralBankRoutes.Post("/participants/provision", middleware.RequireRole(domain.RoleGovernance), deps.ComplianceHandler.ProvisionParticipant)
	centralBankRoutes.Post("/accounts/freeze", middleware.RequireRole(domain.RoleGovernance), deps.ComplianceHandler.FreezeAccount)
	centralBankRoutes.Post("/accounts/unfreeze", middleware.RequireRole(domain.RoleGovernance), deps.ComplianceHandler.UnfreezeAccount)
	centralBankRoutes.Post("/register", middleware.RequireRole(domain.RoleGovernance), deps.ComplianceHandler.RegisterParticipant)

	// --- Governance Portal (Central Bank only) ---
	// Only registered when running as a central bank gateway (no proxy handler).
	// Scope: KYC/onboarding/registry, account control (freeze/unfreeze),
	// circuit breaker, system parameters, and audit logs. Operational actions
	// (mint/burn, deposit/escrow/redeem approvals) live under ROLE_TREASURY.
	//
	// The group guard is the UNION of the roles used inside (spec 042), for the same
	// prefix-scoping reason as centralBankRoutes above. In Scenario A this is not
	// optional: BOTH routes the governance portal uses for onboarding —
	// GET /registry and POST /approve-kyc — live in here, so without the relaxation the
	// Admission profile has no working onboarding surface at all.
	//
	// CONSEQUENCE: the group authorizes nothing by itself. EVERY route below carries an
	// explicit guard; one added without a guard is reachable by BOTH profiles (INV-5).
	if deps.PaymentProxyHandler == nil && deps.GovernanceHandler != nil {
		govGroup := app.Group("/api/v1/governance",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireRole(domain.RoleGovernance, domain.RoleAdmission),
		)
		// Onboarding: approval and participant-record registration are Admission-exclusive;
		// the registry read is shared. These three are why the group guard is a union.
		govGroup.Post("/approve-kyc", middleware.RequireRole(domain.RoleAdmission), deps.GovernanceHandler.ApproveKYC)
		govGroup.Get("/registry", middleware.RequireRole(domain.RoleGovernance, domain.RoleAdmission), deps.GovernanceHandler.GetRegistry)
		govGroup.Post("/participants", middleware.RequireRole(domain.RoleAdmission), deps.GovernanceHandler.RegisterParticipant)

		// Certificate issuance stays with governance: SubmitCSR signs the participant
		// certificate with the central bank's CA key — the same class of act as an
		// on-chain signature, so it is never Admission-authorized (FR-002a).
		govGroup.Post("/registry/csr", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.SubmitCSR)

		// Governance-retained: value/freeze, circuit breaker, parameters, audit, users.
		govGroup.Get("/accounts", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.GetAccounts)
		govGroup.Post("/accounts/freeze", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.FreezeAccount)
		govGroup.Post("/accounts/unfreeze", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.UnfreezeAccount)

		govGroup.Get("/circuit-breaker/status", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.GetCircuitBreakerStatus)
		govGroup.Post("/circuit-breaker/toggle", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.ToggleCircuitBreaker)

		govGroup.Get("/parameters", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.GetParameters)
		govGroup.Put("/parameters", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.UpdateParameters)

		govGroup.Get("/audit/logs", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.GetAuditLogs)

		govGroup.Get("/users", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.ListUsers)
		govGroup.Get("/users/:userId", middleware.RequireRole(domain.RoleGovernance), deps.GovernanceHandler.GetUser)
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

	// --- Treasury: its own audit history (Central Bank only — ROLE_TREASURY) ---
	// The dashboard's operations table reads the audit trail, which lives under the
	// ROLE_GOVERNANCE group above: the operator who performs mint and burn could not read
	// the entries those operations write. This route is the same log pinned to category
	// TREASURY (the handler ignores any category in the query), so it widens who may read
	// the treasury's own operations and nothing else.
	if deps.PaymentProxyHandler == nil && deps.GovernanceHandler != nil {
		treasuryAudit := app.Group("/api/v1/treasury/audit",
			middleware.RequireCookieAuth(deps.AuthProvider),
			middleware.RequireRole(domain.RoleTreasury),
		)
		treasuryAudit.Get("/logs", deps.GovernanceHandler.GetTreasuryAuditLogs)
	}

	// --- Paladin identities (FX agreement party choices) ---
	if deps.IdentityHandler != nil {
		identityGroup := app.Group("/api/v1/identities", middleware.RequireCookieAuth(deps.AuthProvider))
		identityGroup.Get("", deps.IdentityHandler.ListIdentities)

		// Internal gateway-to-gateway endpoint: peer gateways fetch this spoke's
		// LOCAL roster here during roster federation. Protected by the shared
		// X-Relay-Auth secret (a peer gateway has no user cookie), and it returns
		// only local membership so the network-wide lookup does not recurse.
		intIdentities := app.Group("/internal/v1/identities", middleware.RequireRelayAuth(os.Getenv("INTERNAL_RELAY_AUTH_SECRET")))
		intIdentities.Get("", deps.IdentityHandler.ListLocalIdentities)
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
		// HTLC lock/settle/refund are inter-bank payment operations, not oversight
		// actions. Gate the mutating routes to payment-operator roles so an
		// oversight-only session (supervisor/NOC) — or a role-less session that would
		// otherwise fall back to the gateway's own bank id in the counterparty check —
		// cannot drive a settle/refund (R2-H-1 follow-up). Read routes (status/search)
		// stay open to any authenticated caller; the counterparty/bank-ownership check
		// still applies in the handlers. Accepts both the Keycloak realm role
		// (ROLE_BANK) and the compliance form (ROLE_COMMERCIAL_BANK) for banks.
		htlcMutate := middleware.RequireRole(
			domain.RoleBank, domain.RoleCommercialBank, domain.RoleTreasury, domain.RoleGovernance,
		)
		htlcGroup.Post("/lock", htlcMutate, deps.PaymentHandler.LockHTLC)
		htlcGroup.Post("/lock-with-hash", htlcMutate, deps.PaymentHandler.LockHTLCWithHashLock)
		htlcGroup.Post("/settle", htlcMutate, deps.PaymentHandler.SettleHTLC)
		htlcGroup.Post("/refund", htlcMutate, deps.PaymentHandler.RefundHTLC)
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

			// Settled inter-bank PvP legs: settling orchestrators POST each leg here;
			// commercial-bank gateways GET the credits scoped to bank_id to build the
			// credit side of the statement. Central-bank gateway only.
			intPvPLegs := app.Group("/internal/v1/payments/pvp-legs", middleware.RequireRelayAuth(relayAuthSecret))
			intPvPLegs.Post("", deps.PaymentHandler.RecordSettledPvPLeg)

			intPvP := app.Group("/internal/v1/payments/pvp-credits", middleware.RequireRelayAuth(relayAuthSecret))
			intPvP.Get("", deps.PaymentHandler.ListSettledPvPCredits)

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

// csrfExempt reports whether a request is outside the CSRF guard's scope.
//
// Two kinds of route qualify, each for a reason that has to survive being read aloud:
//
//   - The auth entry points run BEFORE a session exists. Login and refresh have no
//     token to present. Logout is exempt for a different reason: it is idempotent and
//     harmless to force, while gating it would strand a user whose CSRF cookie expired
//     before their session did — unable to log out, with no recourse in the UI.
//   - The /internal tree is authenticated by a relay signature, not by a cookie. CSRF
//     defends against AMBIENT credentials the browser attaches on its own; a signed
//     service call carries none.
//
// There is deliberately no exemption for Bearer-authenticated requests: skipping the
// check whenever a caller authenticates by header lets that caller opt out by omitting
// the cookie, and an exemption anyone can select is not a control.
func csrfExempt(c *fiber.Ctx) bool {
	switch c.Path() {
	case "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout":
		return true
	}
	return strings.HasPrefix(c.Path(), "/internal/")
}
