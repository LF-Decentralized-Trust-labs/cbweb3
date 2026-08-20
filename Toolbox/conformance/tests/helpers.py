"""
Shared constants and assertion helpers for the CBWeb3 conformance suite.

Everything in this module is derived from the three published contracts:

  Toolbox/contracts/auth/openapi_auth_v2.3.0.yaml
  Toolbox/contracts/pvp/openapi_pvp_v2.3.0.yaml
  Toolbox/contracts/amm/openapi_amm_v2.3.0.yaml

Nothing here may assert a behaviour the contracts do not document. Where a
behaviour is genuinely undocumented the gap is named in a comment rather than
filled with a plausible guess.
"""

import hashlib
import time

# ---------------------------------------------------------------------------
# HTLC hash-lock pair (Scenario A)
# ---------------------------------------------------------------------------
# THE SECRET IS 32 RANDOM BYTES, HEX-ENCODED — AND THE DIGEST IS TAKEN OVER THE
# DECODED BYTES, NOT OVER THE HEX TEXT.
#
# Verified against the delivered implementation, because the contract's own
# example ("deadbeef01234567...") is elided and its prose says only "pre-image
# whose SHA-256 hash equals the hash lock", which is ambiguous about the encoding:
#
#   payment-orchestrator/internal/grpc/server/server.go:264-270  (lock)
#       secretBytes := make([]byte, 32); rand.Read(secretBytes)
#       secret   = hex.EncodeToString(secretBytes)
#       hashLock = hex.EncodeToString(sha256.Sum256(secretBytes))
#
#   payment-orchestrator/internal/grpc/server/server.go:492-497  (settle)
#       secretBytes, err := hex.DecodeString(req.Secret)
#       if err != nil { return InvalidArgument "invalid secret hex" }   // -> 400
#       hashBytes := sha256.Sum256(secretBytes)
#
# Consequence: a non-hex secret is rejected before any comparison happens. The
# realignment plan §5.4 proposed the pair secret "hello" -> SHA-256("hello");
# that pair is mathematically correct but UNUSABLE against a real gateway on two
# counts — "hello" is not valid hex, and its digest is taken over UTF-8 text
# rather than decoded bytes. The platform surface wins, so the suite uses a
# 32-byte pre-image and the platform's own convention. This is the same pair the
# Toolbox mocks and vectors publish, so all three layers agree.
#
# The previous Toolbox generation published a hash lock for the secret
# "cbweb3-test-secret-2026" and instructed implementers to verify it. The
# published digest was FALSE: the real value is
#   SHA-256("cbweb3-test-secret-2026")
#     = 000cd63c150bad10438f05ca3d5875c25a738017b0b70fc1a539f7f15754fdcf
# and the published one could not be identified as the hash of any pre-image at
# all. Any integrator who followed that instruction got a mismatch. The false
# digest is purged from this repository and is deliberately not reproduced here.
#
# Encoding note, reproduced rather than smoothed: the request field
# `LockHTLCWithHashLockRequest.hash_lock` is described as "hex-encoded, 64 chars"
# and its platform example carries NO 0x prefix (and is 66 characters, a defect
# upstream), while the response field `HTLCLock.hash_lock` example DOES carry a
# 0x prefix. The suite sends the bare 64-character form and never asserts on the
# prefix of what comes back.
HTLC_SECRET = "d2aec5d19f13a018dd1895ef2f97b1cb0ca95664ff69b99bde9303a7a5dda43c"
HTLC_HASH_LOCK = "93d1f1757cd69442489f4f788d90c4dbc37605905c5bf496dea38fadba3ddef2"

# A syntactically valid 32-byte pre-image that hashes to something else. Used to
# exercise the MISMATCH path rather than the hex-parse path.
HTLC_WRONG_SECRET = "1111111111111111111111111111111111111111111111111111111111111111"


def hash_lock_for(secret_hex: str) -> str:
    """
    The platform's hash-lock derivation: SHA-256 over the DECODED bytes of a
    hex-encoded secret, re-encoded as lower-case hex with no 0x prefix.
    """
    return hashlib.sha256(bytes.fromhex(secret_hex)).hexdigest()


# ---------------------------------------------------------------------------
# Timelocks (Scenario A) — SAFETY CRITICAL ORDERING
# ---------------------------------------------------------------------------
# POST /api/v1/htlc/lock           defaults to now + 3600  (initiator)
# POST /api/v1/htlc/lock-with-hash defaults to now + 1800  (responder)
#
# The initiator's lock MUST expire LATER than the responder's, so the responder
# can refund before the initiator can. Nothing in the schema enforces this; it
# exists only in the two `time_lock` descriptions. Any fixture that inverts the
# ordering models an unsafe swap.
INITIATOR_TIMELOCK_OFFSET = 3600
RESPONDER_TIMELOCK_OFFSET = 1800


def initiator_time_lock() -> int:
    return int(time.time()) + INITIATOR_TIMELOCK_OFFSET


def responder_time_lock() -> int:
    return int(time.time()) + RESPONDER_TIMELOCK_OFFSET


# ---------------------------------------------------------------------------
# Vocabularies transcribed verbatim from the contracts
# ---------------------------------------------------------------------------
# pvp: HTLCLock.state — the PREFIXED vocabulary (5 values)
HTLC_LOCK_STATES = {
    "HTLC_STATE_INVALID",
    "HTLC_STATE_PENDING",
    "HTLC_STATE_LOCKED",
    "HTLC_STATE_SETTLED",
    "HTLC_STATE_REFUNDED",
}
# pvp: GET /api/v1/htlc/search `state` QUERY PARAMETER — the BARE vocabulary
# (3 values). The platform never reconciles the two; a client cannot tell which
# to send where. Both are reproduced, neither is normalised.
HTLC_SEARCH_STATES = {"LOCKED", "SETTLED", "REFUNDED"}

# pvp: FXAgreement.state, FXAgreementEvent.from_state/to_state — the PREFIXED
# vocabulary. The delivered gateway serialises the protobuf enum
# (`ag.State.String()`), so what a client receives is `FX_STATE_ACCEPTED`, never
# `ACCEPTED`. The platform OpenAPI document declares the bare values; the
# contract records that divergence and follows the gateway. Do not assert the
# bare set here — no deployment ever emits it.
#
# There is NO EXPIRED state, despite `expiry_date` on every agreement and a
# documented SYSTEM_JOB "background expiration worker" event source. Expiry has
# no representable terminal state. `FX_STATE_INVALID` is the protobuf zero value
# and is representable on the wire.
FX_STATES = {
    "FX_STATE_INVALID",
    "FX_STATE_PROPOSED",
    "FX_STATE_ACCEPTED",
    "FX_STATE_REJECTED",
    "FX_STATE_CANCELLED",
    "FX_STATE_SETTLED",
}
# pvp: listFXAgreements `state` QUERY PARAMETER — the BARE vocabulary, which is
# what the platform document declares and what this suite sends. The delivered
# orchestrator strips an `FX_STATE_` prefix before matching, so both spellings
# are accepted on input; only the output is prefixed.
FX_QUERY_STATES = {"PROPOSED", "ACCEPTED", "REJECTED", "CANCELLED", "SETTLED"}
# pvp: FXAgreementEvent.source — likewise prefixed (`ev.Source.String()`).
# Unlike FXState this enum has no INVALID member.
FX_EVENT_SOURCES = {
    "FX_EVENT_SOURCE_LOCAL_API",
    "FX_EVENT_SOURCE_RELAY",
    "FX_EVENT_SOURCE_SYSTEM_JOB",
    "FX_EVENT_SOURCE_ON_BEHALF",
}

# pvp + amm: DepositRecord/EscrowRecord/RedeemRecord.status
RESERVE_STATES = {"PENDING", "APPROVED", "REJECTED"}

# amm: PoolStatusResponse.pool_status — the one recovered vocabulary the
# contract publishes as a closed enum, because the gateway derives it
# exhaustively from the two reserves and gates swaps on it.
POOL_STATUSES = {"EMPTY", "PENDING_COUNTERPART", "ACTIVE"}

# amm: CrossCurrencySwapStatus.status — documented as OBSERVED values on a free
# string, not as an enum. Used for polling, never asserted as a closed set.
SWAP_TERMINAL_STATES = {"COMPLETED", "FAILED"}

# amm: BridgePositionResult.bridge_state — observed vocabulary only. It lives in
# `internal/domain` and is absent from the platform's OpenAPI, so the contract
# publishes it in prose and NOT as an enum. The suite therefore never asserts
# membership: doing so would assert something the platform has not committed to.
BRIDGE_STATES_OBSERVED = (
    "LOCKING",
    "ACTIVE",
    "BURNING",
    "BURNED",
    "RELEASED",
    "RECONCILIATION_REQUIRED",
)

# ---------------------------------------------------------------------------
# Amounts — always JSON strings, never numbers.
# NO decimals count is stated anywhere for tCeBM/fCeBM on the /api/v1 surface,
# and no unit statement accompanies the 7-digit examples. The /api/v2 surface is
# explicitly 18-decimal base units. The suite therefore uses the contracts' own
# literal example strings and never converts between the two surfaces.
# ---------------------------------------------------------------------------
V1_DEPOSIT_AMOUNT = "10000000"
V1_ESCROW_AMOUNT = "5000000"
V1_HTLC_AMOUNT = "1000000"
V2_AMOUNT_OUT = "1000000000000000000000"
V2_MAX_AMOUNT_IN = "1200000000000000000000"

# Paladin identities, from the contracts' own examples.
PALADIN_SPOKE_A_RECEIVER = "funded_operator@spoke-a-bank-c"
PALADIN_SPOKE_B_RECEIVER = "funded_operator@spoke-b-bank-d"
PALADIN_COUNTERPARTY_B = "funded_operator@spoke-b-bank-d"

# Spoke identifiers and cross-spoke receivers for an FX proposal. The platform
# uses two unreconciled spoke-ID families — `spoke-a`/`spoke-b` in its server and
# identity examples, `spoke-brl`/`spoke-usd` for source_spoke_id/dest_spoke_id.
# These four values are the ones the contract's own propose example carries, and
# the ones pvp_fx_agreement_vectors.json fx-hp-01 uses.
FX_SOURCE_SPOKE_ID = "spoke-brl"
FX_DEST_SPOKE_ID = "spoke-usd"
FX_SOURCE_RECEIVER = "funded_operator@spoke-brl-bank-c"
FX_DEST_RECEIVER = "funded_operator@spoke-usd-bank-b"


# ---------------------------------------------------------------------------
# Assertion / skip helpers
# ---------------------------------------------------------------------------
def status_is(resp, *allowed):
    """Assert an exact status code from the contract's declared response set."""
    assert resp.status_code in allowed, (
        f"{resp.request.method} {resp.request.url} -> {resp.status_code}, "
        f"expected one of {allowed}. Body: {resp.text[:400]}"
    )


def json_body(resp):
    """Parse a JSON body, failing with the raw text when it is not JSON."""
    try:
        return resp.json()
    except ValueError:  # pragma: no cover - diagnostic path
        raise AssertionError(
            f"{resp.request.method} {resp.request.url} -> {resp.status_code} "
            f"returned a non-JSON body: {resp.text[:400]}"
        )


def has_keys(body, *keys):
    """Assert the response object carries every named field."""
    assert isinstance(body, dict), f"expected a JSON object, got {type(body).__name__}"
    missing = [k for k in keys if k not in body]
    assert not missing, f"missing field(s) {missing} in {sorted(body)}"


def is_amount_string(value):
    """Every amount, balance, reserve and limit in both specs is `type: string`."""
    return isinstance(value, str)
