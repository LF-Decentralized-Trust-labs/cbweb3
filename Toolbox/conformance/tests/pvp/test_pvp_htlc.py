"""
Conformance: Scenario A dual-layer HTLC settlement.

Source contract: contracts/pvp/openapi_pvp_v2.3.0.yaml
  POST /api/v1/htlc/lock            (initiator; time_lock defaults to now+3600)
  POST /api/v1/htlc/lock-with-hash  (responder;  time_lock defaults to now+1800)
  POST /api/v1/htlc/settle          (reveals the pre-image)
  POST /api/v1/htlc/refund
  GET  /api/v1/htlc/status/{contractId}
  GET  /api/v1/htlc/search

Scenario B has NO HTLC endpoints at all — the eight HTLC schemas in its platform
document are unreferenced copy-paste residue. Everything here is `scenario_a`.

THE HASH LOCK CROSSES OUT-OF-BAND. There is no API endpoint that transports it:
the initiator locks, reads `hash_lock` from the 201, and conveys it to the
responder by some other channel. Cross-spoke secret propagation is likewise
described in prose with no HTTP endpoint, and HTLC has no relay endpoints
whatsoever. Nothing here invents one.

TWO STATE VOCABULARIES, DELIBERATELY NOT RECONCILED
  HTLCLock.state             HTLC_STATE_LOCKED, ... (prefixed, 5 values)
  /api/v1/htlc/search ?state=       LOCKED, SETTLED, REFUNDED (bare, 3 values)
The platform never reconciles them and a client cannot tell which to send where.
Both are reproduced verbatim; neither is normalised. What the search endpoint
does with a prefixed value is undocumented, so no test asserts it.

NOT TESTED, AND WHY
  * The 403 on the four mutating HTLC routes: the router gates them with
    RequireRole(ROLE_BANK, ROLE_COMMERCIAL_BANK, ROLE_TREASURY, ROLE_GOVERNANCE)
    and our contract declares the 403, but provoking it needs a second session
    holding none of those roles, which a one-credential run cannot provision.
  * `compliance/decrypt-transaction` takes a third HTLC identifier form
    ("0x-prefixed hex") while every HTLC endpoint uses `htlc-a1b2c3d4`. The
    compliance surface is out of scope for this release, so the mismatch is
    recorded, not exercised.
"""

import os
import time

import pytest

from tests.helpers import (
    HTLC_HASH_LOCK,
    HTLC_LOCK_STATES,
    HTLC_SEARCH_STATES,
    HTLC_SECRET,
    HTLC_WRONG_SECRET,
    PALADIN_SPOKE_A_RECEIVER,
    PALADIN_SPOKE_B_RECEIVER,
    V1_HTLC_AMOUNT,
    has_keys,
    initiator_time_lock,
    json_body,
    responder_time_lock,
    hash_lock_for,
    status_is,
)

pytestmark = [pytest.mark.pvp, pytest.mark.htlc, pytest.mark.scenario_a]


@pytest.fixture()
def initiator_lock(session, pvp_url, timeout):
    """
    Lock as the initiator and return (contract_id, hash_lock).

    Against Prism, which cannot persist a lock, the 201 is the contract's static
    example — which still yields a usable contract id, because Prism matches the
    path template rather than the value.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/htlc/lock",
        json={
            "receiver": PALADIN_SPOKE_A_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "time_lock": initiator_time_lock(),
        },
        timeout=timeout,
    )
    status_is(resp, 201)
    body = json_body(resp)
    return body["contract_id"], body["hash_lock"]


# ---------------------------------------------------------------------------
# Fixture-data guards: these assert our own vectors, not the implementation.
# They exist because the previous Toolbox generation shipped a false hash pair
# and instructed integrators to verify it.
# ---------------------------------------------------------------------------
@pytest.mark.edge_case
@pytest.mark.mock_safe
def test_hash_lock_is_the_sha256_of_the_secret():
    """
    HTLC_HASH_LOCK == SHA-256(decoded bytes of HTLC_SECRET), recomputed at run time.

    The derivation follows the delivered implementation exactly: the secret is 32
    random bytes carried as hex, and the digest is taken over the DECODED bytes,
    never over the hex text (server.go:264-270 for lock, :492-497 for settle,
    where a non-hex secret is rejected outright as "invalid secret hex").

    The superseded artefacts published a hash lock for the secret
    "cbweb3-test-secret-2026" that was not its digest — the true value begins
    000cd63c — and told integrators to verify it, so anyone who did got a
    mismatch. This test, plus Toolbox/tools/verify_hashlocks.py, makes the defect
    unrepeatable. The false digest itself is purged from the repository.
    """
    assert hash_lock_for(HTLC_SECRET) == HTLC_HASH_LOCK
    assert len(HTLC_HASH_LOCK) == 64 and len(HTLC_SECRET) == 64
    int(HTLC_HASH_LOCK, 16)  # raises unless it is pure hex
    int(HTLC_SECRET, 16)  # the secret must be hex too, or settle 400s on parse
    assert hash_lock_for(HTLC_WRONG_SECRET) != HTLC_HASH_LOCK


@pytest.mark.edge_case
@pytest.mark.mock_safe
def test_initiator_timelock_outlives_the_responder_timelock():
    """
    SAFETY CRITICAL, AND ENFORCED ONLY BY PROSE.

    The initiator's lock must expire LATER than the responder's, so the responder
    can refund before the initiator can. Nothing in the schema enforces the
    ordering — it exists only in the two `time_lock` descriptions (defaults
    now+3600 and now+1800). A fixture that inverts it models an unsafe swap, so
    the ordering is asserted on our own request data.
    """
    assert initiator_time_lock() > responder_time_lock()


# ---------------------------------------------------------------------------
# Lock
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_initiator_lock_returns_contract_id_and_hash_lock(session, pvp_url, timeout):
    """
    POST /api/v1/htlc/lock -> 201. `contract_id` and `hash_lock` are the two
    REQUIRED response fields: the server generates the secret and returns only
    its digest, which is why the pre-image must reach the responder by another
    channel entirely.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/htlc/lock",
        json={
            "receiver": PALADIN_SPOKE_A_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "time_lock": initiator_time_lock(),
        },
        timeout=timeout,
    )

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "contract_id", "hash_lock")
    assert isinstance(body["contract_id"], str) and body["contract_id"]
    assert isinstance(body["hash_lock"], str) and body["hash_lock"]


@pytest.mark.error
@pytest.mark.mock_safe
def test_lock_without_cookie_returns_401(anon_session, pvp_url, timeout, expect_error):
    """No HTLC operation is public."""
    resp = anon_session.post(
        f"{pvp_url}/api/v1/htlc/lock",
        json={"receiver": PALADIN_SPOKE_A_RECEIVER, "amount": V1_HTLC_AMOUNT},
        timeout=timeout,
    )

    expect_error(resp, 401)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_responder_lock_with_hash_returns_contract_id(session, pvp_url, timeout):
    """
    POST /api/v1/htlc/lock-with-hash -> 201, mirroring the initiator's digest.

    `hash_lock` is sent as bare 64-character hex. The platform's own example for
    this field is 66 characters against its own "64 chars" description, and the
    RESPONSE example carries a 0x prefix the request example lacks — both
    reproduced in the contract with an explicit "do not copy literally" note, and
    neither copied here.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/htlc/lock-with-hash",
        json={
            "receiver": PALADIN_SPOKE_B_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "hash_lock": HTLC_HASH_LOCK,
            "time_lock": responder_time_lock(),
        },
        timeout=timeout,
    )

    status_is(resp, 201)
    has_keys(json_body(resp), "contract_id", "hash_lock")


@pytest.mark.error
@pytest.mark.live_only
def test_lock_with_hash_without_a_hash_lock_returns_400(session, pvp_url, timeout, expect_error):
    """`hash_lock` is required alongside `receiver` and `amount`; 400 is declared."""
    resp = session.post(
        f"{pvp_url}/api/v1/htlc/lock-with-hash",
        json={
            "receiver": PALADIN_SPOKE_B_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "time_lock": responder_time_lock(),
        },
        timeout=timeout,
    )

    expect_error(resp, 400)


# ---------------------------------------------------------------------------
# Status and search
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_status_returns_a_lock_in_the_prefixed_vocabulary(
    session, pvp_url, timeout, initiator_lock
):
    """
    GET /api/v1/htlc/status/{contractId} -> 200, a BARE lock record whose `state`
    is drawn from the PREFIXED five-value enum.

    `Divergence:` the platform OpenAPI document wraps the record as
    `{"lock": {...}}`. The delivered gateway returns it at the top level
    (`c.JSON(result)` over `*HTLCStatus`), and the contract, the mocks and the
    vectors all follow the gateway. `searchHTLC` really does wrap — as
    `{"locks": [...], "total": n}` — which is where the confusion came from.

    The record also has NO `secret` field: the delivered `HTLCStatus` struct does
    not declare one, whatever the platform document says. It carries
    `counterparty_locked`, `amount` and `created_at` instead.
    """
    contract_id, _ = initiator_lock

    resp = session.get(
        f"{pvp_url}/api/v1/htlc/status/{contract_id}", timeout=timeout
    )

    status_is(resp, 200)
    lock = json_body(resp)
    assert "lock" not in lock, (
        "the delivered gateway returns the bare record; a {'lock': ...} wrapper "
        "means the deployment is serving the platform document, not the gateway"
    )
    if "state" in lock:
        assert lock["state"] in HTLC_LOCK_STATES
    if "time_lock" in lock:
        # Unix epoch SECONDS as int64 — never an RFC 3339 string.
        assert isinstance(lock["time_lock"], int)
    if "counterparty_locked" in lock:
        assert isinstance(lock["counterparty_locked"], bool)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_search_accepts_the_bare_state_vocabulary(session, pvp_url, timeout, skip_if_unavailable):
    """
    GET /api/v1/htlc/search?state=LOCKED -> 200 {locks[], total}.

    The query parameter's enum is the BARE vocabulary while the returned
    `HTLCLock.state` uses the PREFIXED one, so a caller filters on LOCKED and
    reads back HTLC_STATE_LOCKED. That is what the platform does; it is not
    normalised here. The declared 502 (Central Bank API unreachable) skips.
    """
    resp = session.get(
        f"{pvp_url}/api/v1/htlc/search", params={"state": "LOCKED"}, timeout=timeout
    )
    skip_if_unavailable(resp)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "locks", "total")
    assert isinstance(body["locks"], list)
    assert "LOCKED" in HTLC_SEARCH_STATES
    for lock in body["locks"]:
        if "state" in lock:
            assert lock["state"] in HTLC_LOCK_STATES


# ---------------------------------------------------------------------------
# Settle
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_settle_with_the_revealed_secret_returns_200(
    session, pvp_url, timeout, require_profile
):
    """
    The responder's leg settles when the initiator reveals the pre-image.

    Built as a self-contained responder lock (our own verified secret/hash pair)
    followed by a settle, because the initiator's lock is created server-side
    with a secret the client never sees — there is no endpoint that reveals it.
    """
    require_profile("scenario-a-bank", "scenario-a-cb")

    lock = session.post(
        f"{pvp_url}/api/v1/htlc/lock-with-hash",
        json={
            "receiver": PALADIN_SPOKE_B_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "hash_lock": HTLC_HASH_LOCK,
            "time_lock": responder_time_lock(),
        },
        timeout=timeout,
    )
    status_is(lock, 201)
    contract_id = json_body(lock)["contract_id"]

    resp = session.post(
        f"{pvp_url}/api/v1/htlc/settle",
        json={"contract_id": contract_id, "secret": HTLC_SECRET},
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    assert "htlc_tx_hash" in body or "zeto_tx_hash" in body

    state = session.get(f"{pvp_url}/api/v1/htlc/status/{contract_id}", timeout=timeout)
    status_is(state, 200)
    # Bare record, not {"lock": {...}} — see
    # test_status_returns_a_lock_in_the_prefixed_vocabulary.
    assert json_body(state)["state"] == "HTLC_STATE_SETTLED"


@pytest.mark.error
@pytest.mark.live_only
def test_settle_with_a_non_matching_secret_does_not_settle(session, pvp_url, timeout):
    """
    A wrong pre-image must not settle the lock.

    HTLC_WRONG_SECRET is a syntactically VALID 32-byte hex secret, so this
    exercises the digest-mismatch path rather than the "invalid secret hex" parse
    rejection that any non-hex string would trigger first.

    The declared response set for this operation is closed — 200, 400, 401, 403,
    500 — so a refusal has to land on 400 or 500. The platform documents no
    machine-readable code (the old HTLC_HASH_MISMATCH / HTLC_EXPIRED codes were
    invented by the superseded Toolbox and exist nowhere on the platform), so
    only the status code is asserted.
    """
    lock = session.post(
        f"{pvp_url}/api/v1/htlc/lock-with-hash",
        json={
            "receiver": PALADIN_SPOKE_B_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "hash_lock": HTLC_HASH_LOCK,
            "time_lock": responder_time_lock(),
        },
        timeout=timeout,
    )
    status_is(lock, 201)

    resp = session.post(
        f"{pvp_url}/api/v1/htlc/settle",
        json={
            "contract_id": json_body(lock)["contract_id"],
            "secret": HTLC_WRONG_SECRET,
        },
        timeout=timeout,
    )

    status_is(resp, 400, 500)


# ---------------------------------------------------------------------------
# Timeout branch
# ---------------------------------------------------------------------------
@pytest.mark.edge_case
@pytest.mark.live_only
def test_refund_after_the_timelock_expires_returns_200(session, pvp_url, timeout):
    """
    The timeout branch: lock with a short timelock, wait it out, refund.

    `time_lock` is the Unix second after which the SENDER can refund. The wait is
    real time and configurable via CBWEB3_REFUND_WAIT_SECONDS (default 20).
    What the platform does NOT document is the status code for refunding BEFORE
    expiry, so that case is deliberately not asserted anywhere in this suite.
    """
    wait = int(os.environ.get("CBWEB3_REFUND_WAIT_SECONDS", "20"))

    lock = session.post(
        f"{pvp_url}/api/v1/htlc/lock-with-hash",
        json={
            "receiver": PALADIN_SPOKE_B_RECEIVER,
            "amount": V1_HTLC_AMOUNT,
            "hash_lock": HTLC_HASH_LOCK,
            "time_lock": int(time.time()) + max(wait - 5, 1),
        },
        timeout=timeout,
    )
    status_is(lock, 201)
    contract_id = json_body(lock)["contract_id"]

    time.sleep(wait)

    resp = session.post(
        f"{pvp_url}/api/v1/htlc/refund",
        json={"contract_id": contract_id},
        timeout=timeout,
    )

    status_is(resp, 200)
    state = session.get(f"{pvp_url}/api/v1/htlc/status/{contract_id}", timeout=timeout)
    status_is(state, 200)
    # Bare record, not {"lock": {...}}.
    assert json_body(state)["state"] == "HTLC_STATE_REFUNDED"


@pytest.mark.error
@pytest.mark.mock_safe
def test_refund_without_cookie_returns_401(anon_session, pvp_url, timeout, expect_error):
    """Refund is a mutating HTLC route and requires a session."""
    resp = anon_session.post(
        f"{pvp_url}/api/v1/htlc/refund",
        json={"contract_id": "htlc-a1b2c3d4"},
        timeout=timeout,
    )

    expect_error(resp, 401)
