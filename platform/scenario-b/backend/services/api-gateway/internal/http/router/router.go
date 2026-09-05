// SPDX-License-Identifier: Apache-2.0

// Package router registers API Gateway routes and attaches required dependencies.
// Scenario B v2 routes are registered by the v2 sub-package (T045/T070/T091).
package router

import (
	"strings"

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
	SupervisorHandler      *handlers.SupervisorHandler
	OnboardingHandler      *handlers.OnboardingHandler
	OnboardingProxyHandler *handlers.OnboardingProxyHandler
	// SpokesHandler serves the internal spoke self-registration endpoint (hub only).
	SpokesHandler *handlers.SpokesHandler
	AuthProvider  interfaces.IAuthProvider
	// PaymentProxyHandler proxies /api/v1/payments/* to the Central Bank gateway.
	// Only wired when CENTRAL_BANK_API_URL is set (commercial bank gateways).
	PaymentProxyHandler *handlers.PaymentProxyHandler
	// PaymentHandler serves /api/v1/token/* endpoints via gRPC to the payment-orchestrator.
	// Only wired when PAYMENT_GRPC_ADDR is set.
	PaymentHandler *handlers.PaymentHandler
	V2Deps         v2router.Dependencies
	// CSRFSecret keys the HMAC binding a CSRF token to its session. When empty the
	// guard still runs and refuses every mutating request, which is the safe way to
	// fail: a gateway that silently stopped enforcing CSRF because a value was
	// missing is the failure this control exists to prevent.
	CSRFSecret []byte
}

// csrfExempt reports whether a request is outside the CSRF guard's scope.
//
// Only two kinds of route qualify, and each has to survive being read aloud:
//
//   - The auth entry points run BEFORE a session exists. Login and refresh have no
//     token to present. Logout is exempt for a different reason: it is idempotent and
//     harmless to force, while gating it would strand a user whose CSRF cookie
//     expired before their session did — unable to log out, with no recourse in the UI.
//   - The /internal tree is authenticated by a relay signature, not by a cookie. CSRF
//     defends against AMBIENT credentials the browser attaches on its own; a signed
//     service call has none, so there is nothing here for a browser to be tricked into.
//
// There is deliberately no exemption for Bearer-authenticated requests. Skipping the
// check whenever a caller authenticates by header lets that caller opt out simply by
// omitting the cookie — an exemption anyone can select is not a control.
func csrfExempt(c *fiber.Ctx) bool {
	switch c.Path() {
	case "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout":
		return true
	}
	return strings.HasPrefix(c.Path(), "/internal/")
}

// Setup registers all gateway HTTP routes and middleware.
func Setup(app *fiber.App, deps Dependencies) {
	// X-Correlation-Id: generated/propagated on ALL requests (Complemento D / NFR-OPS-001).
	app.Use(middleware.CorrelationID())

	// CSRF is mounted app-wide, BEFORE any route is registered, so a route added
	// later is protected without anyone remembering to protect it. Attaching it per
	// group is how the v2 tree ended up with 27 mutating routes and no guard at all.
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
	// Onboarding authorization (spec 042). The read view is shared by the governance
	// and Admission profiles; approving KYC is Admission-exclusive. Both carry
	// per-route guards already, so these are plain guard changes — contrast with
	// govGroup below, whose guard is prefix-scoped.
	complianceGroup.Get("/participants", middleware.RequireRole("ROLE_GOVERNANCE", "ROLE_ADMISSION"), deps.ComplianceHandler.ListParticipants)
	complianceGroup.Post("/approve-kyc", middleware.RequireRole("ROLE_ADMISSION"), deps.GovernanceHandler.ApproveKYC)
	if deps.SupervisorHandler != nil {
		complianceGroup.Get("/audit/logs", middleware.RequireSupervisorRole(), deps.SupervisorHandler.GetAuditLogs)
		complianceGroup.Get("/zk-pointer/verify", middleware.RequireSupervisorRole(), deps.SupervisorHandler.VerifyZKPointer)

		// Read-only participants list for supervisor (same handler, no write access).
		complianceGroup.Get("/participants/summary", middleware.RequireSupervisorRole(), deps.ComplianceHandler.ListParticipants)
	}

	// --- Governance Portal ---
	// The group guard is the UNION of the roles used inside the group, because Fiber
	// group middleware is PREFIX-scoped: a route-level RequireRole runs *in addition*
	// to it, so a narrower group guard would 403 an Admission caller before its route
	// guard ever ran (and re-registering the path at app level does not escape the
	// prefix — only a different path does, see /api/v1/audit/logs below).
	//
	// CONSEQUENCE: the group no longer authorizes anything on its own. EVERY route
	// below must carry its own explicit guard. A route added here without one is
	// reachable by BOTH profiles — see spec 042 INV-5 and the exhaustive table in
	// specs/042-admission-onboarding-profile/contracts/authorization-matrix.md.
	govGroup := app.Group("/api/v1/governance",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireRole("ROLE_GOVERNANCE", "ROLE_ADMISSION"),
	)

	// Onboarding: registering the participant record is Admission-exclusive; the
	// registry read is shared. These two are the reason the group guard is a union.
	govGroup.Post("/participants", middleware.RequireRole("ROLE_ADMISSION"), deps.GovernanceHandler.RegisterParticipant)
	govGroup.Get("/registry", middleware.RequireRole("ROLE_GOVERNANCE", "ROLE_ADMISSION"), deps.GovernanceHandler.GetRegistry)

	// Certificate issuance stays with governance: SubmitCSR signs the participant
	// certificate with the central bank's CA key, which is the same class of act as
	// an on-chain signature. The Admission profile authorizes no central-bank key
	// operation (FR-002a).
	govGroup.Post("/registry/csr", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.SubmitCSR)

	// Governance-retained: value/freeze, circuit breaker, parameters, audit, users.
	// Each needs an explicit guard now that the group guard is a union (FR-004).
	govGroup.Get("/accounts", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.GetAccounts)
	govGroup.Post("/accounts/freeze", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.FreezeAccount)
	govGroup.Post("/accounts/unfreeze", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.UnfreezeAccount)
	govGroup.Get("/circuit-breaker/status", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.GetCircuitBreakerStatus)
	govGroup.Post("/circuit-breaker/toggle", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.ToggleCircuitBreaker)
	govGroup.Get("/parameters", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.GetParameters)
	govGroup.Put("/parameters", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.UpdateParameters)
	govGroup.Get("/audit/logs", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.GetAuditLogs)
	govGroup.Get("/users", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.ListUsers)

	// Audit log READ is shared across the CB portals (treasury/supervisor also consume it).
	// It is served on a dedicated path OUTSIDE the /governance group so the group's
	// ROLE_GOVERNANCE guard does not apply — Fiber group middleware is PREFIX-scoped and
	// would otherwise 403 non-governance callers on any /governance/* path. Read-only:
	// does not weaken any compliance/write control (those stay ROLE_GOVERNANCE-only).
	app.Get("/api/v1/audit/logs",
		middleware.RequireCookieAuth(deps.AuthProvider),
		middleware.RequireRole("ROLE_GOVERNANCE", "ROLE_TREASURY", "ROLE_SUPERVISOR"),
		deps.GovernanceHandler.GetAuditLogs,
	)
	govGroup.Get("/users/:userId", middleware.RequireRole("ROLE_GOVERNANCE"), deps.GovernanceHandler.GetUser)

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

		// Bridge positions come from the central bank too, for the same reason the payment
		// records do: this gateway holds none. An incoming cross-currency delivery is recorded
		// where the burn and release happen, so a beneficiary bank asking its own gateway saw
		// its balance change and no payment at all.
		//
		// Registered HERE, before v2router.Register below, on purpose: the v2 router also
		// serves /api/v2/bridge/positions from this gateway's own (empty) table, and Fiber runs
		// the first matching handler. A bank gateway is exactly the case where the local answer
		// is wrong, and PaymentProxyHandler is non-nil only on a bank gateway.
		app.Get("/api/v2/bridge/positions",
			middleware.RequireCookieAuth(deps.AuthProvider),
			deps.PaymentProxyHandler.ListBridgePositions,
		)
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
		// Signature-preferred, secret-fallback — the same policy as the /internal/amm routes. These
		// carry a commercial bank's deposits, escrows and redeems to its CB, so authenticating them by
		// a secret identical in every entity meant any entity could drive another bank's tokenisation
		// and redemption. The bank's proxy signs them; the secret remains for the migration window.
		internal := app.Group("/internal/v1", middleware.RequireRelayAuthMigrating(deps.V2Deps.RelayAuth))

		internalPayments := internal.Group("/payments")

		// The creation routes are the write half of the same tenant boundary as the listings below,
		// and they were open in the same way: requester_besu_address decides whose record is created
		// and it arrived in the body, which the signature covers but does not attribute. The bank
		// proxy injecting its own address protects honest proxy traffic only — an onboarded bank
		// signs its own calls, so it could POST here naming another bank and have the record created
		// against it. The field is therefore derived from the verified identity, exactly as
		// requester_id is on the reads.
		//
		// The fiat exchange names no address to overwrite: it names a deposit, so it is bound by
		// whose deposit that is.
		scopeBodyToCaller := middleware.ScopeRequesterBodyToCaller(deps.V2Deps.RequesterScopeResolver)
		internalPayments.Post("/deposits/exchange",
			middleware.BindDepositToCaller(deps.V2Deps.RequesterScopeResolver, deps.PaymentHandler),
			deps.PaymentHandler.RequestFiatExchange)
		internalPayments.Post("/deposits", scopeBodyToCaller, deps.PaymentHandler.RegisterDeposit)
		internalPayments.Post("/escrows", scopeBodyToCaller, deps.PaymentHandler.RequestEscrow)
		internalPayments.Post("/redeems", scopeBodyToCaller, deps.PaymentHandler.RequestRedeem)
		// The listing handlers are shared with the CB's own /api/v1/payments routes, where returning the
		// whole book is the point. Here the caller is a single commercial bank, so requester_id is the
		// tenant boundary — and it is therefore taken from the identity whose signature was verified,
		// not from the query string, which no signature covers. A bank that signs a listing request and
		// attaches another bank's address gets its own records back, and a caller with no verified
		// identity gets none.
		scopeToCaller := middleware.ScopeRequesterToCaller(deps.V2Deps.RequesterScopeResolver)
		internalPayments.Get("/deposits", scopeToCaller, deps.PaymentHandler.ListDeposits)
		internalPayments.Get("/escrows", scopeToCaller, deps.PaymentHandler.ListEscrows)
		internalPayments.Get("/redeems", scopeToCaller, deps.PaymentHandler.ListRedeems)

		// One bank's bridge positions, so a beneficiary can see the payment it received. The
		// delivery position is created here, on the CB's gateway; the bank's own gateway holds
		// none, so the receiving institution saw a balance change and no payment at all.
		//
		// Registered inside this group, and WITHOUT its own auth middleware, because the group
		// already authenticates every /internal/v1 request. A second RequireRelayAuth here runs
		// the replay guard twice over one request: the first pass records the signature, the
		// second sees it recorded and rejects the call. Live, that made the route answer
		// RELAY_SIGNATURE_REPLAYED to its very first caller.
		//
		// The owner is scoped inside the handler rather than by ScopeRequesterToCaller: that
		// middleware binds requester_id, an ADDRESS, while positions are keyed by owner_bank_id
		// — the entity id whose signature was verified. Same rule, different key.
		if deps.V2Deps.BridgePositionReader != nil {
			bridgePositions := handlers.NewBridgeHandler(nil, nil, deps.V2Deps.BridgePositionReader)
			internal.Get("/bridge/positions", bridgePositions.ListPositionsForCaller)
		}
	}

	// --- Internal spoke self-registration (hub only) ---
	// A founding central bank registers its spoke on the hub IdentityRegistry via
	// the hub compliance service. Machine-to-machine, guarded by X-Relay-Auth.
	if deps.SpokesHandler != nil {
		relaySecret := os.Getenv("INTERNAL_RELAY_AUTH_SECRET")
		spokes := app.Group("/internal/v1", middleware.RequireRelayAuth(relaySecret))
		spokes.Post("/spokes/register", deps.SpokesHandler.RegisterSpoke)
		spokes.Post("/spokes/register-currency", deps.SpokesHandler.RegisterSpokeCurrency)
		spokes.Post("/spokes/register-pair", deps.SpokesHandler.RegisterSpokePair)
	}

	// --- Scenario B API v2 ---
	v2router.Register(app, deps.V2Deps)
}
