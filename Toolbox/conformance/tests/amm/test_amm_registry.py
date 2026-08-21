"""
Conformance: Scenario B discovery surface (hub registries, pools, breaker).

Source contract: contracts/amm/openapi_amm_v2.3.0.yaml

These are the twelve operations an integrator can call with NO credentials at
all — the only public surface on the whole platform outside /healthz and the auth
endpoints. Everything here therefore runs on `anon_session`.

RESPONSE-ENVELOPE DIVERGENCES, MIRRORED FROM THE IMPLEMENTATION
`hub/currencies`, `amm/pairs` and `bridge/positions` are documented upstream as
bare ARRAYS but actually return wrapped objects — {"currencies": [...]},
{"pairs": [...]}, {"positions": [...]}. Our contract models the wrapped form and
these tests assert it. A client written strictly against the upstream document
would break on all three.

NEVER CONSTRUCT A POOL-PAIR IDENTIFIER. Five incompatible spellings coexist
across the platform artefacts — `tCeBM_BRL-tCeBM_ARS`, `W-tCeBM_BRL/W-tCeBM_ARS`,
`W-BRL-ARS`, the quote-style `W-BRL-W-ARS` and the circuit breaker's `BRL-USD`
fallback — and none is marked canonical. All five are enumerated with source
citations in Toolbox/DIVERGENCES.md. `pair_id` from GET /api/v2/amm/pairs is the
only supported source; treat it as opaque.

NOT TESTED, AND WHY
  * Circuit-breaker pause / resume-request / resume-sign, hub currency
    registration and pair propose/confirm are governance mutations that halt a
    live corridor or write to a shared registry. They are exercised only through
    the corridor-provisioning fixture in test_amm_swap.py, under an explicit
    Central Bank profile, and never as standalone tests.
  * `signResume` answers 200 with `state: RESUME_PENDING` when the on-chain
    execution fails, so a 200 there does not mean trading resumed. Any future
    test must branch on `state`, not on the status code.
"""

from urllib.parse import quote

import pytest

from tests.helpers import POOL_STATUSES, has_keys, is_amount_string, json_body, status_is

pytestmark = [pytest.mark.amm, pytest.mark.registry, pytest.mark.scenario_b]


@pytest.fixture(scope="module")
def pool_pair(anon_session, amm_url, timeout):
    """
    An opaque pair identifier obtained from the registry — never constructed.

    Skips the dependent tests when the registry is empty: a gateway with no
    corridor provisioned is a legitimate deployment state, not a defect.
    """
    resp = anon_session.get(f"{amm_url}/api/v2/amm/pairs", timeout=timeout)
    if resp.status_code != 200:
        pytest.skip(f"GET /api/v2/amm/pairs -> {resp.status_code}")
    pairs = resp.json().get("pairs") or []
    if not pairs:
        pytest.skip("no AMM pair registered on this gateway")
    return pairs[0]["pair_id"]


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_hub_config_is_public(anon_session, amm_url, timeout):
    """GET /api/v2/amm/hub-config -> 200 with the three Hub contract addresses."""
    resp = anon_session.get(f"{amm_url}/api/v2/amm/hub-config", timeout=timeout)

    status_is(resp, 200)
    has_keys(json_body(resp), "amm_address", "pair_registry", "currency_registry")


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_hub_liquidity_config_is_public(anon_session, amm_url, timeout):
    """
    GET /api/v2/amm/hub-liquidity-config -> 200.

    `default_sovereign_pool_pair` falls back to a literal `W-BRL-ARS` — the
    sovereign spelling, one of the five catalogued in Toolbox/DIVERGENCES.md. It
    is read, never parsed.
    """
    resp = anon_session.get(f"{amm_url}/api/v2/amm/hub-liquidity-config", timeout=timeout)
    if resp.status_code == 404:
        pytest.skip("sovereign liquidity is not configured on this gateway")

    status_is(resp, 200)
    assert isinstance(json_body(resp), dict)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_hub_currencies_returns_a_wrapped_object(anon_session, amm_url, timeout):
    """GET /api/v2/hub/currencies -> 200 {"currencies": [...]}, not a bare array."""
    resp = anon_session.get(f"{amm_url}/api/v2/hub/currencies", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "currencies")
    assert isinstance(body["currencies"], list)
    for currency in body["currencies"]:
        has_keys(currency, "symbol")


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_pairs_returns_a_wrapped_object_with_pair_ids(anon_session, amm_url, timeout):
    """
    GET /api/v2/amm/pairs -> 200 {"pairs": [...]}, each carrying `pair_id`.

    `pair_id` is the only supported source of a `pool_pair` value anywhere in the
    Scenario B surface.
    """
    resp = anon_session.get(f"{amm_url}/api/v2/amm/pairs", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "pairs")
    assert isinstance(body["pairs"], list)
    for pair in body["pairs"]:
        has_keys(pair, "pair_id")
        assert isinstance(pair["pair_id"], str) and pair["pair_id"]


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_pool_status_uses_the_documented_enum(anon_session, amm_url, timeout, pool_pair):
    """
    GET /api/v2/amm/pool/{pair}/status -> 200.

    `pool_status` is the one recovered vocabulary the contract publishes as a
    closed enum — EMPTY | PENDING_COUNTERPART | ACTIVE — because the gateway
    derives it exhaustively from the two reserves and gates swaps on it. Reserves
    are strings in 18-decimal base units on this surface.

    UNDOCUMENTED TRAP, FOUND BY RUNNING THIS TEST: the `pair_id` the registry
    returns contains a forward slash (`W-tCeBM_BRL/W-tCeBM_ARS`), so dropping it
    verbatim into the `{pair}` path segment produces a URL that matches no route.
    It MUST be percent-encoded. Nothing in either platform spec says so, and the
    same identifier goes UNENCODED into `pool_pair` request bodies and query
    parameters — same value, two different treatments depending on where it
    lands.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/pool/{quote(pool_pair, safe='')}/status",
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "pool_pair", "pool_status")
    assert body["pool_status"] in POOL_STATUSES
    for reserve in ("reserve_a", "reserve_b"):
        if reserve in body:
            assert is_amount_string(body[reserve])


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_circuit_breaker_status_is_public_and_pair_scoped(
    anon_session, amm_url, timeout, pool_pair
):
    """
    GET /api/v2/governance/circuit-breaker/status?pair=... -> 200.

    THE `pair` PARAMETER IS A TRAP AND IS SENT DELIBERATELY: the platform
    document declares no parameter at all, while the handler defaults to the
    literal `BRL-USD`. Omitting it can return a healthy LIVE for a pair nobody
    trades. `state` is a free string (observed: LIVE, HALTED, RESUME_PENDING) and
    is not asserted as a closed set, because the platform has not committed to
    one.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/governance/circuit-breaker/status",
        params={"pair": pool_pair},
        timeout=timeout,
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "state")
    assert isinstance(body["state"], str) and body["state"]


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_hub_token_supply_declares_decimals(anon_session, amm_url, timeout, skip_if_unavailable):
    """
    GET /api/v2/hub/token/supply -> 200 {symbol, token_address, total_supply, decimals}.

    `decimals` here is the ONLY place in the entire CBWeb3 surface where a
    decimals value is asserted by the platform. Nothing equivalent exists for
    tCeBM/fCeBM on /api/v1, which is why no scale factor is ever assumed when
    moving a figure between the two surfaces.
    """
    resp = anon_session.get(f"{amm_url}/api/v2/hub/token/supply", timeout=timeout)
    skip_if_unavailable(resp)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "total_supply")
    assert is_amount_string(body["total_supply"])
    if "decimals" in body:
        assert isinstance(body["decimals"], int)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_lp_balance_is_public(anon_session, amm_url, timeout):
    """GET /api/v2/amm/lp-balance -> 200 {lp_shares, lp_total_supply}, both strings."""
    resp = anon_session.get(f"{amm_url}/api/v2/amm/lp-balance", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "lp_shares", "lp_total_supply")
    for field in ("lp_shares", "lp_total_supply"):
        if field in body:
            assert is_amount_string(body[field])


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_liquidity_positions_are_public(anon_session, amm_url, timeout, pool_pair):
    """
    GET /api/v2/amm/liquidity/positions?pool_pair=... -> 200 {pool_pair, positions, count}.

    Unauthenticated: each gateway publishes only its own positions.

    `pool_pair` is MANDATORY. The platform document declares no query parameters
    at all and a bare array response; the delivered handler answers 400
    INVALID_REQUEST without `pool_pair` and wraps the result in an envelope. A
    gateway with no LP repository wired answers 501 NOT_IMPLEMENTED, which is a
    deployment fact and not a conformance failure.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/liquidity/positions",
        params={"pool_pair": pool_pair},
        timeout=timeout,
    )
    if resp.status_code == 501:
        pytest.skip("LP position listing is not enabled on this gateway")

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "pool_pair", "positions", "count")
    assert isinstance(body["positions"], list)
    assert body["count"] == len(body["positions"])
    for pos in body["positions"]:
        # provider_bank_id, NOT provider_id — the DTO differs from every other
        # provider-keyed shape on this surface.
        if "provider_id" in pos:
            raise AssertionError(
                "positions are keyed provider_bank_id; a provider_id means the "
                "deployment is serving something other than LPPositionDTO"
            )


@pytest.mark.error
@pytest.mark.mock_safe
def test_liquidity_positions_without_pool_pair_returns_400(
    anon_session, amm_url, timeout
):
    """
    Omitting `pool_pair` is a 400 with `code: INVALID_REQUEST` — not an empty
    list. Undeclared in the platform document; observed in the delivered handler
    and declared in the contract.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/liquidity/positions", timeout=timeout
    )
    if resp.status_code == 501:
        pytest.skip("LP position listing is not enabled on this gateway")

    status_is(resp, 400)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_disclosure_status_is_public_in_scenario_b(anon_session, amm_url, timeout):
    """
    GET /api/v2/oversight/disclosure-status/{requestID} is `security: []` in
    Scenario B, while its Scenario A counterpart requires a session. Asserted
    because it is a genuine and surprising asymmetry between the two gateways.
    A 404 for an unknown id is the declared outcome and is accepted here.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/oversight/disclosure-status/"
        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
        timeout=timeout,
    )

    status_is(resp, 200, 404)
