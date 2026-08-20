"""
Conformance: Scenario A reserve lifecycle (token + deposits/escrows/redeems).

Source contract: contracts/pvp/openapi_pvp_v2.3.0.yaml

Why a PvP suite tests reserves at all: a bank cannot HTLC-lock tokens it does not
hold. tCeBM exists only after deposit -> Central Bank approve -> escrow (Reserve
Tokenisation) -> Central Bank approve. Without this prelude an HTLC lock can never
execute and the suite degrades into shape-checking a mock.

SCENARIO A SPELLING, DO NOT GENERALISE: the fiat conversion endpoint here is
POST /api/v1/payments/deposits/fiat-exchange. Scenario B's equivalent is
POST /api/v1/payments/deposits/exchange. The two gateways genuinely differ.

NOT TESTED, AND WHY
  * No decimals count is stated anywhere for tCeBM/fCeBM on the /api/v1 surface.
    The suite uses the contract's own literal example amounts and never converts
    a figure between the /api/v1 and /api/v2 surfaces.
  * Balances are opaque strings; no arithmetic beyond "non-zero" is performed.
"""

import pytest

from tests.helpers import (
    RESERVE_STATES,
    V1_DEPOSIT_AMOUNT,
    V1_ESCROW_AMOUNT,
    has_keys,
    is_amount_string,
    json_body,
    status_is,
)

pytestmark = [pytest.mark.pvp, pytest.mark.reserves, pytest.mark.scenario_a]


@pytest.fixture()
def registered_deposit(session, pvp_url, timeout):
    """Register a deposit and return its id. Used by the approval tests."""
    resp = session.post(
        f"{pvp_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )
    status_is(resp, 201)
    return json_body(resp)["deposit_id"]


@pytest.fixture()
def requested_escrow(session, pvp_url, timeout):
    """Request Reserve Tokenisation and return the escrow id."""
    resp = session.post(
        f"{pvp_url}/api/v1/payments/escrows",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )
    status_is(resp, 201)
    return json_body(resp)["escrow_id"]


# ---------------------------------------------------------------------------
# Deposits
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_register_deposit_returns_201_with_deposit_id(session, pvp_url, timeout):
    """
    POST /api/v1/payments/deposits -> 201 {deposit_id, status}.

    `amount` is the only required field. `requester_besu_address` and
    `requester_paladin_identity` are proxy-injected from the verified caller on a
    commercial-bank gateway — a value the client sends is overwritten, so the
    suite never sends caller identity in the body.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "deposit_id")
    assert isinstance(body["deposit_id"], str) and body["deposit_id"]
    if "status" in body:
        assert body["status"] in RESERVE_STATES


@pytest.mark.error
@pytest.mark.mock_safe
def test_register_deposit_without_cookie_returns_401(
    anon_session, pvp_url, timeout, expect_error
):
    """Every Scenario A operation is CookieAuth; none is public."""
    resp = anon_session.post(
        f"{pvp_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )

    expect_error(resp, 401)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_deposits_returns_deposits_and_total(session, pvp_url, timeout):
    """
    GET /api/v1/payments/deposits -> 200 {deposits[], total}.

    Unpaginated: there is no page/limit parameter on this operation, no cursor
    and no Link header. `total` is a count, not a pagination control.
    """
    resp = session.get(f"{pvp_url}/api/v1/payments/deposits", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "deposits", "total")
    assert isinstance(body["deposits"], list)
    assert isinstance(body["total"], int)
    for record in body["deposits"]:
        if "status" in record:
            assert record["status"] in RESERVE_STATES
        if "amount" in record:
            assert is_amount_string(record["amount"])


@pytest.mark.happy_path
@pytest.mark.live_only
def test_approve_deposit_returns_201(
    session, pvp_url, timeout, registered_deposit, require_profile, skip_if_unavailable
):
    """
    POST /api/v1/payments/deposits/approve -> 201 {status}.

    Registered only on a Central Bank gateway (PaymentProxyHandler == nil), so a
    commercial-bank profile skips rather than fails.
    """
    require_profile("scenario-a-cb")

    resp = session.post(
        f"{pvp_url}/api/v1/payments/deposits/approve",
        json={"deposit_id": registered_deposit},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "status")


@pytest.mark.error
@pytest.mark.live_only
def test_reject_deposit_echoes_the_reason(
    session, pvp_url, timeout, registered_deposit, require_profile, skip_if_unavailable
):
    """POST /api/v1/payments/deposits/reject -> 200 {reason} (stored as rejection_reason)."""
    require_profile("scenario-a-cb")
    reason = "funds not received by settlement cut-off"

    resp = session.post(
        f"{pvp_url}/api/v1/payments/deposits/reject",
        json={"deposit_id": registered_deposit, "reason": reason},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 200)
    has_keys(json_body(resp), "reason")


# ---------------------------------------------------------------------------
# Escrows — Reserve Tokenisation
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_request_escrow_returns_201_with_escrow_id(session, pvp_url, timeout):
    """POST /api/v1/payments/escrows -> 201 {escrow_id, status}: fCeBM -> tCeBM."""
    resp = session.post(
        f"{pvp_url}/api/v1/payments/escrows",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "escrow_id")
    if "status" in body:
        assert body["status"] in RESERVE_STATES


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_escrows_returns_escrows_and_total(session, pvp_url, timeout):
    """GET /api/v1/payments/escrows -> 200 {escrows[], total}."""
    resp = session.get(f"{pvp_url}/api/v1/payments/escrows", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "escrows", "total")
    assert isinstance(body["escrows"], list)
    assert isinstance(body["total"], int)


@pytest.mark.happy_path
@pytest.mark.live_only
def test_approve_escrow_returns_burn_and_mint_hashes(
    session, pvp_url, timeout, requested_escrow, require_profile, skip_if_unavailable
):
    """
    POST /api/v1/payments/escrows/approve -> 201 {burn_tx_hash, mint_tx_hash}.

    Two hashes because Reserve Tokenisation is two on-chain acts: the fCeBM is
    burned and the tCeBM is minted into Zeto.

    `Divergence:` the platform OpenAPI document names the second field
    `zeto_mint_tx_hash`. The delivered Scenario A gateway returns
    `ApproveEscrowResult`, whose JSON tags are `burn_tx_hash` and `mint_tx_hash`,
    passed straight to `c.JSON` — so `zeto_mint_tx_hash` never reaches a client.
    This is a document-vs-implementation split inside Scenario A, not a
    Scenario A vs Scenario B difference: Scenario B uses `mint_tx_hash` too.
    """
    require_profile("scenario-a-cb")

    resp = session.post(
        f"{pvp_url}/api/v1/payments/escrows/approve",
        json={"escrow_id": requested_escrow},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 201)
    has_keys(json_body(resp), "burn_tx_hash", "mint_tx_hash")


# ---------------------------------------------------------------------------
# Balances
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_token_balance_is_a_string(session, pvp_url, timeout):
    """
    GET /api/v1/token/balance -> 200 {balance}.

    Every amount in both platform specs is `type: string` — "stored as string for
    precision". Never a JSON number.
    """
    resp = session.get(f"{pvp_url}/api/v1/token/balance", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "balance")
    assert is_amount_string(body["balance"])


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_fiat_balance_is_a_string(session, pvp_url, timeout, skip_if_unavailable):
    """
    GET /api/v1/token/fiat-balance -> 200 {balance}.

    Declares a 503 for "Fiat token adapter unavailable", which is a deployment
    outcome and skips rather than fails.
    """
    resp = session.get(f"{pvp_url}/api/v1/token/fiat-balance", timeout=timeout)
    skip_if_unavailable(resp)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "balance")
    assert is_amount_string(body["balance"])


@pytest.mark.happy_path
@pytest.mark.live_only
def test_reserve_prelude_yields_a_non_zero_tcebm_balance(
    session, pvp_url, timeout, require_profile, skip_if_unavailable
):
    """
    The full prelude: deposit -> approve -> escrow -> approve -> balance > 0.

    This is the settlement-grade assertion that a shape test cannot make. It runs
    only on a Central Bank gateway, because the two approve halves are registered
    only there.
    """
    require_profile("scenario-a-cb")

    deposit = session.post(
        f"{pvp_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )
    status_is(deposit, 201)
    deposit_id = json_body(deposit)["deposit_id"]

    approved = session.post(
        f"{pvp_url}/api/v1/payments/deposits/approve",
        json={"deposit_id": deposit_id},
        timeout=timeout,
    )
    skip_if_unavailable(approved)
    status_is(approved, 201)

    escrow = session.post(
        f"{pvp_url}/api/v1/payments/escrows",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )
    status_is(escrow, 201)
    escrow_id = json_body(escrow)["escrow_id"]

    tokenised = session.post(
        f"{pvp_url}/api/v1/payments/escrows/approve",
        json={"escrow_id": escrow_id},
        timeout=timeout,
    )
    skip_if_unavailable(tokenised)
    status_is(tokenised, 201)

    balance = session.get(f"{pvp_url}/api/v1/token/balance", timeout=timeout)
    status_is(balance, 200)
    value = json_body(balance)["balance"]
    assert is_amount_string(value)
    # Base-10 ASCII integer, arbitrary precision, unit unstated by the platform.
    assert int(value) > 0, "Reserve Tokenisation completed but tCeBM balance is zero"


@pytest.mark.happy_path
@pytest.mark.live_only
def test_fiat_exchange_returns_a_mint_tx_hash(
    session, pvp_url, timeout, registered_deposit, require_profile, skip_if_unavailable
):
    """
    POST /api/v1/payments/deposits/fiat-exchange -> 201 {mint_tx_hash}.

    `Divergence:` the platform OpenAPI document names the field `tx_hash`. The
    delivered Scenario A gateway returns `FiatExchangeResult`, whose single JSON
    tag is `mint_tx_hash`, passed straight to `c.JSON`. Scenario B agrees with the
    gateway; only the document disagrees with both.

    The PATH, by contrast, really is scenario-specific: Scenario A serves
    `/deposits/fiat-exchange`, Scenario B serves `/deposits/exchange`.
    """
    require_profile("scenario-a-cb")

    approved = session.post(
        f"{pvp_url}/api/v1/payments/deposits/approve",
        json={"deposit_id": registered_deposit},
        timeout=timeout,
    )
    skip_if_unavailable(approved)
    status_is(approved, 201)

    resp = session.post(
        f"{pvp_url}/api/v1/payments/deposits/fiat-exchange",
        json={"deposit_id": registered_deposit},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 201)
    has_keys(json_body(resp), "mint_tx_hash")


# ---------------------------------------------------------------------------
# Redeems
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_redeems_returns_redeems_and_total(session, pvp_url, timeout):
    """
    GET /api/v1/payments/redeems -> 200 {redeems[], total}.

    Creating a redeem is not exercised as a mock-safe test: on Scenario A the
    commercial-bank proxy performs a real tCeBM transfer to the Central Bank
    before forwarding the request, so a create moves value and is not safe to
    replay against a shared estate. (The `zeto_transfer_tx_hash` the platform
    document asks a client for is produced by that transfer and injected by the
    proxy — it is never supplied by the caller.)
    """
    resp = session.get(f"{pvp_url}/api/v1/payments/redeems", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "redeems", "total")
    assert isinstance(body["redeems"], list)
    assert isinstance(body["total"], int)


@pytest.mark.error
@pytest.mark.live_only
def test_redeem_without_an_amount_returns_400(
    session, pvp_url, timeout, expect_error
):
    """
    `amount` is the only field a client supplies to this route, and omitting it is
    the one 400 the operation raises.

    `Do not assert on a missing zeto_transfer_tx_hash.` The public route is served
    by the commercial-bank proxy, which performs the tCeBM transfer itself and
    injects `zeto_transfer_tx_hash`, `requester_besu_address` and
    `requester_paladin_identity` before forwarding. Omitting the hash succeeds; the
    platform document presents it as a client obligation, and it is not one.

    live_only because Prism answers a schema violation with its own 422 envelope,
    not the gateway's 400.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/payments/redeems",
        json={},
        timeout=timeout,
    )

    expect_error(resp, 400)
