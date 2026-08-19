// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	besuscanner "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/besu"
	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	paymentadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/payment"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TransferLimitChecker gates outgoing HTLC locks against CB-configured daily limits.
type TransferLimitChecker interface {
	CheckAndDeductTransferLimit(ctx context.Context, payerBankID, currency, amountHuman string) (allowed bool, errorCode, maxAmount string, err error)
	RestoreTransferLimit(ctx context.Context, payerBankID, currency, amountHuman string) error
}

// ParticipantResolver resolves registered participants so deposit/escrow/redeem
// listings can surface the requesting institution's name instead of a raw address.
type ParticipantResolver interface {
	ListParticipants(ctx context.Context, statusFilter, search string) ([]complianceadapter.Participant, error)
}

// AuditLogWriter persists a single audit entry. Mint and burn create and destroy
// money, and until R2-M-8 neither left any record of who did it, for how much or
// why — which is also why the treasury history screen, reading the TREASURY
// category, had nothing to show. Writing is best effort: see recordTokenAudit.
type AuditLogWriter interface {
	CreateAuditLog(ctx context.Context, entry complianceadapter.AuditEntry) error
}

// PvPLedger persists settled inter-bank PvP legs at the Central Bank and serves
// each bank the legs on which it is the receiver (the credit side of its
// statement). *services.PvPLedgerService implements it. Only the central-bank
// gateway wires this in.
type PvPLedger interface {
	RecordLeg(ctx context.Context, in services.SettledLegInput) error
	ListCreditsForBank(ctx context.Context, bankID string) ([]services.PvPCreditRow, error)
}

// PaymentHandler exposes the payment-orchestrator operations as REST endpoints.
type PaymentHandler struct {
	payment      *paymentadapter.GRPCAdapter
	bankCode     string // institution fallback when JWT lacks BankID (e.g. user not yet registered in compliance)
	htlcScanner  *besuscanner.HTLCScanner
	fiatSymbol   string // currency for transfer limit checks (e.g. "BRL", "ARS"); empty = skip check
	limitChecker TransferLimitChecker
	participants ParticipantResolver // optional; enables requester-name enrichment on listings
	// identityRoster resolves the Paladin identities valid as FX agreement
	// parties (from real Pente membership, or an explicit override). When set and
	// non-empty, ProposeFXAgreement rejects any party identity not in it with a
	// 400 (fail fast, before the on-chain Pente propose).
	identityRoster *IdentityRoster
	// rosterRetryInitial/rosterRetryMax bound the fail-closed retry of roster
	// resolution at propose time: on a resolution error we retry with exponential
	// backoff (starting at rosterRetryInitial) until the roster resolves or the
	// window (min(request deadline, rosterRetryMax)) elapses, then reject.
	rosterRetryInitial time.Duration
	rosterRetryMax     time.Duration
	// pvpLedger persists/serves settled inter-bank PvP legs. Central-bank gateway only.
	pvpLedger PvPLedger
	// auditLogger records mint/burn in the immutable audit trail. Optional: when
	// nil the handlers behave exactly as they did before R2-M-8, so an entity
	// without compliance wired is unaffected.
	auditLogger AuditLogWriter
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(payment *paymentadapter.GRPCAdapter, bankCode string) *PaymentHandler {
	return &PaymentHandler{
		payment:            payment,
		bankCode:           bankCode,
		rosterRetryInitial: 100 * time.Millisecond,
		rosterRetryMax:     15 * time.Second,
	}
}

// WithParticipantResolver attaches a compliance participant resolver used to
// enrich deposit/escrow/redeem listings with the requester's institution name.
func (h *PaymentHandler) WithParticipantResolver(resolver ParticipantResolver) *PaymentHandler {
	h.participants = resolver
	return h
}

// WithAuditLogger attaches the compliance audit writer used to record mint and
// burn. Without it those operations leave no trail at all.
func (h *PaymentHandler) WithAuditLogger(writer AuditLogWriter) *PaymentHandler {
	h.auditLogger = writer
	return h
}

// WithIdentityRoster configures the Paladin identities accepted as FX agreement
// parties. An empty roster disables validation (nothing to validate against).
func (h *PaymentHandler) WithIdentityRoster(roster *IdentityRoster) *PaymentHandler {
	h.identityRoster = roster
	return h
}

// WithPvPLedger attaches the Central Bank PvP ledger used to ingest settled legs
// and serve each bank its incoming credits. Central-bank gateway only.
func (h *PaymentHandler) WithPvPLedger(ledger PvPLedger) *PaymentHandler {
	h.pvpLedger = ledger
	return h
}

// resolveRosterForPropose resolves the FX-party roster for propose-time
// validation, fail-closed: on a resolution error it retries with exponential
// backoff (honoring ctx) until the roster resolves or the retry window elapses,
// then returns the error so the caller rejects the trade rather than reaching
// the on-chain propose blind. Returns (nil, nil) when no roster is configured
// (validation disabled). A successfully-resolved empty roster is not an error.
func (h *PaymentHandler) resolveRosterForPropose(parent context.Context) ([]string, error) {
	if h.identityRoster == nil {
		return nil, nil
	}

	ctx := parent
	// Bound the retry window even when the request context carries no deadline,
	// so a persistently-unavailable roster cannot hang the request forever.
	if _, hasDeadline := parent.Deadline(); !hasDeadline && h.rosterRetryMax > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, h.rosterRetryMax)
		defer cancel()
	}

	backoff := h.rosterRetryInitial
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	const maxBackoff = time.Second

	for attempt := 1; ; attempt++ {
		roster, err := h.identityRoster.Identities(ctx)
		if err == nil {
			return roster, nil
		}
		log.Printf("warning: FX party roster resolution attempt %d failed: %v", attempt, err)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("roster unresolved after %d attempt(s): %w", attempt, err)
		case <-timer.C:
		}
		if backoff < maxBackoff {
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// unknownIdentities returns the non-empty identities, in the given order and
// de-duplicated, that are not present in roster. An empty roster means
// "nothing to validate against" and yields no unknowns.
func unknownIdentities(roster []string, identities ...string) []string {
	if len(roster) == 0 {
		return nil
	}
	valid := make(map[string]struct{}, len(roster))
	for _, id := range roster {
		valid[strings.TrimSpace(id)] = struct{}{}
	}
	var invalid []string
	seen := make(map[string]struct{}, len(identities))
	for _, id := range identities {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, dup := seen[trimmed]; dup {
			continue
		}
		seen[trimmed] = struct{}{}
		if _, ok := valid[trimmed]; !ok {
			invalid = append(invalid, trimmed)
		}
	}
	return invalid
}

// participantNamesByWallet builds a lowercase-wallet-address → institution-name
// map from the compliance registry. Returns nil when no resolver is configured
// or the lookup fails; enrichment is best-effort and never blocks a listing.
func (h *PaymentHandler) participantNamesByWallet(ctx context.Context) map[string]string {
	if h.participants == nil {
		return nil
	}
	participants, err := h.participants.ListParticipants(ctx, "", "")
	if err != nil {
		log.Printf("[payment] WARNING: requester-name enrichment skipped, participant lookup failed: %v", err)
		return nil
	}
	names := make(map[string]string, len(participants))
	for _, p := range participants {
		if p.WalletAddress == "" || p.InstitutionName == "" {
			continue
		}
		names[strings.ToLower(p.WalletAddress)] = p.InstitutionName
	}
	return names
}

// enrichScanResultNames fills SenderName/ReceiverName on each scan result by
// matching the (checksummed) EVM addresses against a lowercase-wallet → name
// map. No-op when names is nil/empty; unmatched addresses stay unnamed.
func enrichScanResultNames(results []besuscanner.HTLCScanResult, names map[string]string) {
	if len(names) == 0 {
		return
	}
	for i := range results {
		results[i].SenderName = names[strings.ToLower(results[i].Sender)]
		results[i].ReceiverName = names[strings.ToLower(results[i].Receiver)]
	}
}

// SetHTLCScanner wires an on-chain scanner; when set, supervisor HTLC searches bypass the orchestrator.
func (h *PaymentHandler) SetHTLCScanner(s *besuscanner.HTLCScanner) {
	h.htlcScanner = s
}

// WithLimitChecker attaches a transfer-limit checker and the fiat currency symbol.
func (h *PaymentHandler) WithLimitChecker(checker TransferLimitChecker, fiatSymbol string) *PaymentHandler {
	h.limitChecker = checker
	h.fiatSymbol = fiatSymbol
	return h
}

// --- HTLC endpoints ---

func (h *PaymentHandler) LockHTLC(c *fiber.Ctx) error {
	var req struct {
		AgreementID string `json:"agreement_id"`
		Receiver    string `json:"receiver"`
		Amount      string `json:"amount"`
		TimeLock    uint64 `json:"time_lock"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Receiver == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "receiver and amount are required"})
	}
	// Apply smart defaults
	if req.AgreementID == "" {
		req.AgreementID = uuid.NewString()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 3600 // #nosec G115 -- time.Now().Unix() is always ≥0; overflow not reachable
	}
	if ok, err := h.checkAndDeductLimit(c, req.Amount); !ok {
		return err
	}
	result, err := h.payment.LockHTLC(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock)
	if err != nil {
		h.restoreLimit(c, req.Amount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) LockHTLCWithHashLock(c *fiber.Ctx) error {
	var req struct {
		AgreementID string `json:"agreement_id"`
		Receiver    string `json:"receiver"`
		Amount      string `json:"amount"`
		TimeLock    uint64 `json:"time_lock"`
		HashLock    string `json:"hash_lock"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.HashLock == "" || req.Receiver == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "hash_lock, receiver and amount are required"})
	}
	// Apply smart defaults
	if req.AgreementID == "" {
		req.AgreementID = uuid.NewString()
	}
	if req.TimeLock == 0 {
		req.TimeLock = uint64(time.Now().Unix()) + 1800 // #nosec G115 -- time.Now().Unix() is always ≥0; overflow not reachable
	}
	if ok, err := h.checkAndDeductLimit(c, req.Amount); !ok {
		return err
	}
	result, err := h.payment.LockHTLCWithHashLock(c.Context(), req.AgreementID, req.Receiver, req.Amount, req.TimeLock, req.HashLock)
	if err != nil {
		h.restoreLimit(c, req.Amount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) SettleHTLC(c *fiber.Ctx) error {
	var req struct {
		ContractID string `json:"contract_id"`
		Secret     string `json:"secret"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	// Only an HTLC counterparty may settle it, and the orchestrator re-checks via
	// x-caller-identity. Reject unauthenticated callers and third parties (R2-H-9/H-10).
	ctx, ok := h.authorizeHTLCCounterparty(c, req.ContractID)
	if !ok {
		return nil
	}
	result, err := h.payment.SettleHTLC(ctx, req.ContractID, req.Secret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) RefundHTLC(c *fiber.Ctx) error {
	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	// Only an HTLC counterparty may refund it, and the orchestrator re-checks via
	// x-caller-identity. Reject unauthenticated callers and third parties (R2-H-9/H-10).
	ctx, ok := h.authorizeHTLCCounterparty(c, req.ContractID)
	if !ok {
		return nil
	}
	result, err := h.payment.RefundHTLC(ctx, req.ContractID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// authorizeHTLCCounterparty enforces that the authenticated caller is a
// counterparty (sender or receiver) of the given HTLC before a mutating action
// (settle/refund). It reads the JWT claims, propagates x-caller-identity to the
// orchestrator (which re-checks server-side), fetches the HTLC and verifies
// counterparty membership. On any failure it writes the appropriate response
// (401 / 403 / 500) and returns ok=false; on success it returns the outgoing
// context carrying x-caller-identity and ok=true. Fails closed.
func (h *PaymentHandler) authorizeHTLCCounterparty(c *fiber.Ctx, contractID string) (context.Context, bool) {
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
		return nil, false
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	lock, err := h.payment.GetHTLCStatus(ctx, contractID)
	if err != nil {
		if st, ok2 := status.FromError(err); ok2 && st.Code() == codes.PermissionDenied {
			_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
			return nil, false
		}
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		return nil, false
	}
	counterparty, parseErr := isHTLCCounterparty(lock.Sender, lock.Receiver, callerBankID)
	if parseErr != nil {
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "identity format error: " + parseErr.Error()})
		return nil, false
	}
	if !counterparty {
		_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
		return nil, false
	}
	return ctx, true
}

func (h *PaymentHandler) GetHTLCStatus(c *fiber.Ctx) error {
	contractID := c.Params("contractId")
	if contractID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "contractId is required"})
	}

	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}

	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	result, err := h.payment.GetHTLCStatus(ctx, contractID)
	if err != nil {
		if st, ok2 := status.FromError(err); ok2 && st.Code() == codes.PermissionDenied {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	counterparty, parseErr := isHTLCCounterparty(result.Sender, result.Receiver, callerBankID)
	if parseErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "identity format error: " + parseErr.Error()})
	}
	if !counterparty {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a counterparty of this HTLC"})
	}

	return c.JSON(result)
}

func (h *PaymentHandler) SearchHTLC(c *fiber.Ctx) error {
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	// Supervisors have network-wide read access.
	isSupervisor := false
	for _, r := range claims.Roles {
		if r == domain.RoleSupervisor {
			isSupervisor = true
			break
		}
	}

	// Supervisor + on-chain scanner: bypass the payment-orchestrator entirely so all
	// HTLCs on the network are visible, not just those indexed by this entity's orchestrator.
	if isSupervisor && h.htlcScanner != nil {
		scanResults, err := h.htlcScanner.ScanAllHTLCs(c.UserContext())
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "on-chain scan failed: " + err.Error()})
		}
		enrichScanResultNames(scanResults, h.participantNamesByWallet(c.UserContext()))
		return c.JSON(fiber.Map{"locks": scanResults, "total": len(scanResults)})
	}

	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}

	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	results, err := h.payment.SearchHTLC(ctx,
		c.Query("agreement_id"),
		c.Query("sender"),
		c.Query("receiver"),
		c.Query("state"),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if isSupervisor {
		// Scanner not configured: fall back to orchestrator results (partial view).
		return c.JSON(fiber.Map{"locks": results, "total": len(results)})
	}

	// Keep only records where the caller's institution is a counterparty.
	filtered := results[:0]
	for _, r := range results {
		counterparty, parseErr := isHTLCCounterparty(r.Sender, r.Receiver, callerBankID)
		if parseErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "identity format error: " + parseErr.Error()})
		}
		if counterparty {
			filtered = append(filtered, r)
		}
	}

	return c.JSON(fiber.Map{"locks": filtered, "total": len(filtered)})
}

// isHTLCCounterparty reports whether bankID (from JWT claims) exactly matches
// the institution embedded in either Paladin identity (format: name@spoke-{prefix}-{bankID}).
// Exact segment matching is required: "bank-a" must not match "bank-abc".
// Parse errors are returned so callers can log them; on error the check fails closed.
func isHTLCCounterparty(sender, receiver, bankID string) (bool, error) {
	if bankID == "" {
		return false, nil
	}
	senderBank, sErr := bankIDFromIdentity(sender)
	receiverBank, rErr := bankIDFromIdentity(receiver)
	if sErr != nil {
		return false, sErr
	}
	if rErr != nil {
		return false, rErr
	}
	return senderBank == bankID || receiverBank == bankID, nil
}

// bankIDFromIdentity extracts the bank identifier from a Paladin identity string.
// e.g. "funded_operator@spoke-a-bank-a" → "bank-a"
//
// The Paladin identity format "{name}@{spoke-word}-{letter}-{bankID}" is structural
// to this function: the bankID is the third dash-delimited segment after the "@".
// If Paladin changes this naming convention, this function will return an error and
// all authorization checks will fail closed until the implementation is updated.
// Mirrors identity.BankID in the payment-orchestrator; keep both in sync or move to a shared module.
func bankIDFromIdentity(paladinIdentity string) (string, error) {
	parts := strings.SplitN(paladinIdentity, "@", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("bankIDFromIdentity: missing '@' in %q — expected format {name}@{spoke-word}-{letter}-{bankID}", paladinIdentity)
	}
	segs := strings.SplitN(parts[1], "-", 3)
	if len(segs) < 3 {
		return "", fmt.Errorf("bankIDFromIdentity: fewer than three dash-segments in %q — expected format spoke-{letter}-{bankID}", parts[1])
	}
	return segs[2], nil
}

// --- Token endpoints ---

func (h *PaymentHandler) MintToken(c *fiber.Ctx) error {
	var req struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
		// RequestID and ReserveProofRef are the operator's stated backing for the
		// issuance. They are recorded when present but NOT required: the livehappy
		// E2E, the performance provisioning script and the FX tryout all mint as
		// scenario setup, with no deposit to reference.
		RequestID       string `json:"request_id"`
		ReserveProofRef string `json:"reserve_proof_ref"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.MintToken(c.Context(), req.To, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.recordTokenAudit(c, "TOKEN_MINT", req.To, fmt.Sprintf(
		`{"amount":%q,"to":%q,"request_id":%q,"reserve_proof_ref":%q}`,
		req.Amount, req.To, req.RequestID, req.ReserveProofRef))
	return c.Status(fiber.StatusCreated).JSON(result)
}

// recordTokenAudit writes one entry for a completed mint or burn.
//
// Best effort by design: the on-chain operation has already happened and cannot
// be rolled back, so failing the response would report a false negative to the
// operator. Blocking on it would also let a compliance outage stop money
// operations that work today. The failure is logged, never swallowed. Category
// is TREASURY because that is what the treasury history screen reads.
func (h *PaymentHandler) recordTokenAudit(c *fiber.Ctx, action, target, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := ""
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok {
		actor = claims.Subject
	}
	if err := h.auditLogger.CreateAuditLog(c.UserContext(), complianceadapter.AuditEntry{
		ActorSubject:  actor,
		ActorAddress:  c.IP(),
		IPAddress:     c.IP(),
		ActionType:    action,
		TargetSubject: target,
		Result:        "SUCCESS",
		Category:      "TREASURY",
		Severity:      "HIGH",
		Details:       details,
	}); err != nil {
		log.Printf("[payment] %s audit log write failed (non-fatal): %v", action, err)
	}
}

func (h *PaymentHandler) BurnToken(c *fiber.Ctx) error {
	var req struct {
		From   string `json:"from"`
		Amount string `json:"amount"`
		// Reason is the operator's justification for destroying money. The screen
		// has always demanded it, but it used to be dropped in the browser, so the
		// stated reason existed nowhere on the server. It is required here so it
		// cannot be lost again.
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.From == "" || req.Amount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from and amount are required"})
	}
	if strings.TrimSpace(req.Reason) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "reason is required"})
	}
	result, err := h.payment.BurnToken(c.Context(), req.From, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	h.recordTokenAudit(c, "TOKEN_BURN", req.From, fmt.Sprintf(
		`{"amount":%q,"from":%q,"reason":%q}`, req.Amount, req.From, strings.TrimSpace(req.Reason)))
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) TransferToken(c *fiber.Ctx) error {
	var req struct {
		To     string `json:"to"`
		Amount string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	// A transfer debits the caller's own balance: derive the payer identity from
	// the authenticated claims and propagate it so the orchestrator scopes the
	// transfer to the caller rather than trusting the request blindly (R2-H-9/H-10).
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)
	result, err := h.payment.TransferToken(ctx, req.To, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetBalance(c *fiber.Ctx) error {
	result, err := h.payment.GetBalance(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetFiatBalance(c *fiber.Ctx) error {
	result, err := h.payment.GetFiatBalance(c.Context())
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// --- Escrow: Deposit endpoints ---

func (h *PaymentHandler) RegisterDeposit(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RegisterDeposit(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveDeposit(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.ApproveDeposit(c.Context(), req.DepositID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "approved"})
}

func (h *PaymentHandler) RejectDeposit(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
		Reason    string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectDeposit(c.Context(), req.DepositID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) RequestFiatExchange(c *fiber.Ctx) error {
	var req struct {
		DepositID string `json:"deposit_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestFiatExchange(c.Context(), req.DepositID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ListDeposits(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	deposits, err := h.payment.ListDeposits(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if names := h.participantNamesByWallet(c.Context()); names != nil {
		for i := range deposits {
			deposits[i].RequesterName = names[strings.ToLower(deposits[i].RequesterBesuAddress)]
		}
	}
	return c.JSON(fiber.Map{"deposits": deposits, "total": len(deposits)})
}

// --- Escrow: Tokenization endpoints ---

func (h *PaymentHandler) RequestEscrow(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestEscrow(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveEscrow(c *fiber.Ctx) error {
	var req struct {
		EscrowID string `json:"escrow_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.ApproveEscrow(c.Context(), req.EscrowID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) RejectEscrow(c *fiber.Ctx) error {
	var req struct {
		EscrowID string `json:"escrow_id"`
		Reason   string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectEscrow(c.Context(), req.EscrowID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) ListEscrows(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	escrows, err := h.payment.ListEscrows(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if names := h.participantNamesByWallet(c.Context()); names != nil {
		for i := range escrows {
			escrows[i].RequesterName = names[strings.ToLower(escrows[i].RequesterBesuAddress)]
		}
	}
	return c.JSON(fiber.Map{"escrows": escrows, "total": len(escrows)})
}

// --- Escrow: Redeem endpoints ---

func (h *PaymentHandler) RequestRedeem(c *fiber.Ctx) error {
	var req struct {
		RequesterBesuAddress     string `json:"requester_besu_address"`
		RequesterPaladinIdentity string `json:"requester_paladin_identity"`
		Amount                   string `json:"amount"`
		ZetoTransferTxHash       string `json:"zeto_transfer_tx_hash"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.RequestRedeem(c.Context(), req.RequesterBesuAddress, req.RequesterPaladinIdentity, req.Amount, req.ZetoTransferTxHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) ApproveRedeem(c *fiber.Ctx) error {
	var req struct {
		RedeemID string `json:"redeem_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	result, err := h.payment.ApproveRedeem(c.Context(), req.RedeemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) RejectRedeem(c *fiber.Ctx) error {
	var req struct {
		RedeemID string `json:"redeem_id"`
		Reason   string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.payment.RejectRedeem(c.Context(), req.RedeemID, req.Reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"reason": req.Reason})
}

func (h *PaymentHandler) ListRedeems(c *fiber.Ctx) error {
	requesterID := c.Query("requester_id")
	redeems, err := h.payment.ListRedeems(c.Context(), requesterID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if names := h.participantNamesByWallet(c.Context()); names != nil {
		for i := range redeems {
			redeems[i].RequesterName = names[strings.ToLower(redeems[i].RequesterBesuAddress)]
		}
	}
	return c.JSON(fiber.Map{"redeems": redeems, "total": len(redeems)})
}

// --- FX Agreement endpoints ---

func (h *PaymentHandler) ProposeFXAgreement(c *fiber.Ctx) error {
	var req struct {
		TradeID         string `json:"trade_id"`
		CounterpartyB   string `json:"counterparty_b"`
		Originator      string `json:"originator"`
		SettlementAgent string `json:"settlement_agent"`
		Custodian       string `json:"custodian"`
		Beneficiary     string `json:"beneficiary"`
		OriginAmount    string `json:"origin_amount"`
		CounterAmount   string `json:"counter_amount"`
		OriginCurrency  string `json:"origin_currency"`
		CounterCurrency string `json:"counter_currency"`
		Rate            string `json:"rate"`
		ExpiryDate      uint64 `json:"expiry_date"`
		SourceSpokeID   string `json:"source_spoke_id"`
		DestSpokeID     string `json:"dest_spoke_id"`
		SourceReceiver  string `json:"source_receiver"`
		DestReceiver    string `json:"dest_receiver"`
		OnBehalf        bool   `json:"on_behalf"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CounterpartyB == "" || req.OriginAmount == "" || req.CounterAmount == "" ||
		req.OriginCurrency == "" || req.CounterCurrency == "" || req.Rate == "" || req.ExpiryDate == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "counterparty_b, origin_amount, counter_amount, origin_currency, counter_currency, rate, and expiry_date are required"})
	}
	// Reject any party identity that is not a real Pente member before reaching
	// the on-chain propose, which would otherwise fail with a cryptic Pente
	// membership error (PD011814). Fail-closed: if the roster cannot be resolved
	// (orchestrator unreachable) we retry until it resolves or the window
	// elapses, then reject with 503 rather than propose without verifying
	// membership.
	roster, rosterErr := h.resolveRosterForPropose(c.UserContext())
	if rosterErr != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "could not verify party membership: Paladin roster unavailable, please retry: " + rosterErr.Error(),
		})
	}
	if invalid := unknownIdentities(
		roster,
		req.CounterpartyB,
		req.SettlementAgent,
		req.Custodian,
		req.Beneficiary,
		req.SourceReceiver,
		req.DestReceiver,
	); len(invalid) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":              "one or more party identities are not members of the Paladin roster: " + strings.Join(invalid, ", "),
			"invalid_identities": invalid,
		})
	}
	// Propagate the authenticated caller identity so the orchestrator binds the
	// originator to the caller and rejects a client-supplied foreign originator
	// (R2-H-9/H-10). on_behalf is a governance/relay operation (direct gRPC) and is
	// never honored from this user-facing route.
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	result, err := h.payment.ProposeFXAgreement(ctx, &pb.ProposeFXAgreementRequest{
		TradeId:         req.TradeID,
		CounterpartyB:   req.CounterpartyB,
		Originator:      req.Originator,
		SettlementAgent: req.SettlementAgent,
		Custodian:       req.Custodian,
		Beneficiary:     req.Beneficiary,
		OriginAmount:    req.OriginAmount,
		CounterAmount:   req.CounterAmount,
		OriginCurrency:  req.OriginCurrency,
		CounterCurrency: req.CounterCurrency,
		Rate:            req.Rate,
		ExpiryDate:      req.ExpiryDate,
		SourceSpokeId:   req.SourceSpokeID,
		DestSpokeId:     req.DestSpokeID,
		SourceReceiver:  req.SourceReceiver,
		DestReceiver:    req.DestReceiver,
		OnBehalf:        false,
	})
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *PaymentHandler) AcceptFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	// on_behalf is a governance operation (on-chain acceptOnBehalf, GOVERNANCE-only)
	// driven by the CB relay over a direct gRPC call — never through this
	// user-facing route. Do NOT read it from the client body: honoring it here would
	// let any authenticated caller bypass the initiator≠acceptor check (R2-H-9/H-10).
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	result, err := h.payment.AcceptFXAgreement(ctx, tradeID, false)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) RejectFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	// on_behalf is a governance operation (on-chain rejectOnBehalf, GOVERNANCE-only)
	// driven by the CB relay over a direct gRPC call — never through this
	// user-facing route. Do NOT read it from the client body (R2-H-9/H-10).
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	result, err := h.payment.RejectFXAgreement(ctx, tradeID, false)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) CancelFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	// Require authentication and propagate the caller identity so the orchestrator
	// enforces that only a party to the agreement may cancel it (R2-H-9/H-10).
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	result, err := h.payment.CancelFXAgreement(ctx, tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) SettleFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	// Require authentication and propagate the caller identity so the orchestrator
	// enforces that only a party to the agreement may settle it (R2-H-9/H-10).
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok || claims.Subject == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	callerBankID := claims.BankID
	if callerBankID == "" {
		callerBankID = h.bankCode
	}
	ctx := metadata.AppendToOutgoingContext(c.Context(), "x-caller-identity", callerBankID)

	result, err := h.payment.SettleFXAgreement(ctx, tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(result)
}

func (h *PaymentHandler) GetFXAgreement(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	result, err := h.payment.GetFXAgreement(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"agreement": result})
}

func (h *PaymentHandler) ListFXAgreements(c *fiber.Ctx) error {
	results, err := h.payment.ListFXAgreements(c.Context(), c.Query("counterparty"), c.Query("state"))
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"agreements": results, "total": len(results)})
}

func (h *PaymentHandler) ListFXAgreementEvents(c *fiber.Ctx) error {
	tradeID := c.Params("tradeId")
	if tradeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tradeId is required"})
	}
	results, err := h.payment.ListFXAgreementEvents(c.Context(), tradeID)
	if err != nil {
		return grpcErrorToHTTP(c, err)
	}
	return c.JSON(fiber.Map{"events": results, "total": len(results)})
}

// RecordSettledPvPLeg ingests a settled inter-bank PvP leg reported by a settling
// orchestrator and persists it in the Central Bank's PvP ledger. It is an
// internal, relay-authenticated endpoint served only by the central-bank gateway.
// The leg is keyed by contract_id so settle retries / relay redelivery upsert
// idempotently. The receiver bank id is parsed here from the receiver identity.
//
//	POST /internal/v1/payments/pvp-legs
//	  { "trade_id","contract_id","sender","receiver","amount","settled_at" }
func (h *PaymentHandler) RecordSettledPvPLeg(c *fiber.Ctx) error {
	if h.pvpLedger == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "pvp ledger not configured"})
	}
	var body struct {
		TradeID    string `json:"trade_id"`
		ContractID string `json:"contract_id"`
		Sender     string `json:"sender"`
		Receiver   string `json:"receiver"`
		Amount     string `json:"amount"`
		SettledAt  string `json:"settled_at"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.ContractID == "" || body.Receiver == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "contract_id and receiver are required"})
	}
	receiverBankID, err := bankIDFromIdentity(body.Receiver)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unparseable receiver identity: " + err.Error()})
	}
	settledAt, err := time.Parse(time.RFC3339, body.SettledAt)
	if err != nil {
		settledAt = time.Now().UTC() // tolerate a missing/invalid timestamp rather than reject the leg
	}
	if err := h.pvpLedger.RecordLeg(c.Context(), services.SettledLegInput{
		ContractID:     body.ContractID,
		TradeID:        body.TradeID,
		Sender:         body.Sender,
		Receiver:       body.Receiver,
		ReceiverBankID: receiverBankID,
		Amount:         body.Amount,
		SettledAt:      settledAt,
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

// ListSettledPvPCredits returns the incoming inter-bank PvP settlement legs on
// which the given bank is the receiver, read from the Central Bank's PvP ledger
// (populated by settling orchestrators, keyed on the same HTLC-settlement event
// that produces the sender's debit). This is an internal, relay-authenticated
// endpoint served only by the central-bank gateway. Commercial-bank gateways
// consume it to build the credit side of the statement: a receiving bank's own
// orchestrator holds no record of an incoming leg (the counterparty locked it on
// a different orchestrator, and the amount is private Zeto value). Results are
// scoped to bank_id so a bank never sees legs it is not party to. Legs are
// denominated in tCeBM (reserve value) at the statement layer.
//
//	GET /internal/v1/payments/pvp-credits?bank_id=bank-c
//	  -> { "credits": [ { "reference": "<contractId>", "amount": "700", "settled_at": "..." } ], "total": N }
func (h *PaymentHandler) ListSettledPvPCredits(c *fiber.Ctx) error {
	bankID := strings.TrimSpace(c.Query("bank_id"))
	if bankID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bank_id is required"})
	}
	if h.pvpLedger == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "pvp ledger not configured"})
	}
	rows, err := h.pvpLedger.ListCreditsForBank(c.Context(), bankID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	credits := make([]PvPCredit, 0, len(rows))
	for _, r := range rows {
		credits = append(credits, PvPCredit{Reference: r.Reference, Amount: r.Amount, SettledAt: r.SettledAt})
	}
	return c.JSON(fiber.Map{"credits": credits, "total": len(credits)})
}

// checkAndDeductLimit enforces the CB daily transfer limit before an HTLC lock.
// Returns (true, nil) when allowed or no checker configured.
// Returns (false, nil) after writing a 422/503 response — callers must return nil to Fiber.
func (h *PaymentHandler) checkAndDeductLimit(c *fiber.Ctx, amount string) (bool, error) {
	if h.limitChecker == nil || h.fiatSymbol == "" {
		return true, nil
	}
	allowed, errorCode, _, err := h.limitChecker.CheckAndDeductTransferLimit(c.Context(), h.bankCode, h.fiatSymbol, amount)
	if err != nil {
		return false, c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":      "transfer limit service unavailable",
			"error_code": "LIMIT_SERVICE_UNAVAILABLE",
		})
	}
	if !allowed {
		return false, c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":              "daily transfer limit exceeded",
			"error_code":         errorCode,
			"recommended_action": "Contact your Central Bank to review or increase the daily transfer limit.",
		})
	}
	return true, nil
}

// restoreLimit is best-effort: called when an HTLC lock fails after a successful deduction.
func (h *PaymentHandler) restoreLimit(c *fiber.Ctx, amount string) {
	if h.limitChecker == nil || h.fiatSymbol == "" {
		return
	}
	_ = h.limitChecker.RestoreTransferLimit(c.Context(), h.bankCode, h.fiatSymbol, amount)
}

// grpcErrorToHTTP maps gRPC status codes to appropriate HTTP responses.
func grpcErrorToHTTP(c *fiber.Ctx, err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case codes.InvalidArgument:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case codes.FailedPrecondition:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case codes.PermissionDenied:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	case codes.Unavailable:
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}
