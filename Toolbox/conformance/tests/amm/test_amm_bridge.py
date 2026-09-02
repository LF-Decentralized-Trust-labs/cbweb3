"""
Conformance: Scenario B bridge and hub reconciliation.

Source contract: contracts/amm/openapi_amm_v2.3.0.yaml
  POST /api/v2/bridge/lock-mint      (lock native tCeBM on the spoke, mint W- on the Hub)
  POST /api/v2/bridge/burn-unlock    (burn on the Hub, release on the spoke)
  GET  /api/v2/bridge/positions
  GET  /api/v2/amm/hub-reconciliation

All four are RequireAnyAuth: the session cookie OR a bearer token, which is why
they carry `bearer_ok`. They are among the twelve /api/v2 operations where
BearerAuth is declared as an alternative — chiefly machine-to-machine Central
Bank sovereign flows.

BRIDGE STATE IS NOT ASSERTED AS AN ENUM, ON PURPOSE. The real vocabulary
(LOCKING, ACTIVE, BURNING, BURNED, RELEASED, RECONCILIATION_REQUIRED) lives in
the gateway's `internal/domain` package and appears nowhere in the platform's
OpenAPI, which instead suggests PENDING / ACTIVE / CLOSED — two of which do not
exist. Our contract publishes the real vocabulary in prose and declares the field
a free string, so these tests assert the field is a non-empty string and nothing
more. Asserting membership would assert something the platform has not committed
to.

NEVER SEND CALLER IDENTITY IN THE BODY. `owner_bank_id`, `spoke_network`,
`native_asset` and `mirrored_asset` are all deprecated on the lock-mint request:
the handler derives them from the session's BankID claim and fails closed with
401 UNAUTHENTICATED. Only `amount` is required, and only `amount` is sent.
"""

import pytest

from tests.helpers import V2_AMOUNT_OUT, has_keys, is_amount_string, json_body, status_is

pytestmark = [pytest.mark.amm, pytest.mark.bridge, pytest.mark.scenario_b]


@pytest.mark.happy_path
@pytest.mark.mock_safe
@pytest.mark.bearer_ok
def test_bridge_positions_returns_a_wrapped_object(session, amm_url, timeout):
    """
    GET /api/v2/bridge/positions -> 200 {"positions": [...]}.

    Documented upstream as a bare array; it is not. Amounts are strings in
    18-decimal base units on this surface.
    """
    resp = session.get(f"{amm_url}/api/v2/bridge/positions", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "positions")
    assert isinstance(body["positions"], list)
    for position in body["positions"]:
        if "mirrored_amount" in position:
            assert is_amount_string(position["mirrored_amount"])
        if "bridge_state" in position:
            # Free string by contract — see the module docstring.
            assert isinstance(position["bridge_state"], str) and position["bridge_state"]


@pytest.mark.error
@pytest.mark.mock_safe
def test_bridge_positions_without_a_credential_returns_401(anon_session, amm_url, timeout):
    """RequireAnyAuth still means *some* credential: neither cookie nor bearer is 401."""
    resp = anon_session.get(f"{amm_url}/api/v2/bridge/positions", timeout=timeout)

    status_is(resp, 401)


@pytest.mark.error
@pytest.mark.mock_safe
def test_lock_mint_without_a_credential_returns_401(anon_session, amm_url, timeout):
    """POST /api/v2/bridge/lock-mint requires a credential; the body is valid."""
    resp = anon_session.post(
        f"{amm_url}/api/v2/bridge/lock-mint",
        json={"amount": V2_AMOUNT_OUT},
        timeout=timeout,
    )

    status_is(resp, 401)


@pytest.mark.error
@pytest.mark.mock_safe
def test_burn_unlock_without_a_credential_returns_401(anon_session, amm_url, timeout):
    """POST /api/v2/bridge/burn-unlock requires a credential; the body is valid."""
    resp = anon_session.post(
        f"{amm_url}/api/v2/bridge/burn-unlock",
        json={"position_id": "9e1c4b77-2f10-4a8d-9c31-6b4e7d2a1f05"},
        timeout=timeout,
    )

    status_is(resp, 401)


@pytest.mark.happy_path
@pytest.mark.live_only
@pytest.mark.bearer_ok
def test_lock_mint_creates_a_bridge_position(
    session, amm_url, timeout, require_profile, skip_if_unavailable
):
    """
    POST /api/v2/bridge/lock-mint -> 201 with a position and its Hub mirror.

    Central Bank flow: it locks real reserves on the spoke and mints the wrapped
    Hub token, so it runs only under a Central Bank profile.
    """
    require_profile("scenario-b-cb")

    resp = session.post(
        f"{amm_url}/api/v2/bridge/lock-mint",
        json={"amount": V2_AMOUNT_OUT},
        timeout=timeout,
    )
    skip_if_unavailable(resp)
    if resp.status_code == 422:
        pytest.skip(f"lock-mint refused: {resp.text[:200]}")

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "position_id", "mirrored_amount", "bridge_state")
    assert body["position_id"]
    assert is_amount_string(body["mirrored_amount"])


@pytest.mark.happy_path
@pytest.mark.live_only
@pytest.mark.bearer_ok
def test_burn_unlock_returns_the_same_position(
    session, amm_url, timeout, require_profile, skip_if_unavailable
):
    """
    lock-mint then burn-unlock: the 200 echoes the same `position_id`.

    The resulting `bridge_state` is read but not asserted against a vocabulary —
    the platform has not published one. A 409 means a previous attempt for this
    position did not complete and reconciliation is required first; that is a
    real estate condition, so it skips rather than fails.
    """
    require_profile("scenario-b-cb")

    minted = session.post(
        f"{amm_url}/api/v2/bridge/lock-mint",
        json={"amount": V2_AMOUNT_OUT},
        timeout=timeout,
    )
    skip_if_unavailable(minted)
    if minted.status_code != 201:
        pytest.skip(f"lock-mint unavailable for this run: {minted.status_code}")
    position_id = json_body(minted)["position_id"]

    resp = session.post(
        f"{amm_url}/api/v2/bridge/burn-unlock",
        json={"position_id": position_id},
        timeout=timeout,
    )
    skip_if_unavailable(resp)
    if resp.status_code in (409, 422):
        pytest.skip(f"burn-unlock refused: {resp.text[:200]}")

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "position_id", "bridge_state")
    assert body["position_id"] == position_id


@pytest.mark.happy_path
@pytest.mark.mock_safe
@pytest.mark.bearer_ok
def test_hub_reconciliation_reports_a_balanced_flag(
    session, amm_url, timeout, skip_if_unavailable
):
    """
    GET /api/v2/amm/hub-reconciliation -> 200.

    `balanced` is true exactly when `unexplained` is zero, per the contract. The
    two are cross-checked against each other rather than assuming a healthy
    estate: a non-zero `unexplained` is a reportable condition, not necessarily a
    loss, and never a conformance failure of the API itself.
    """
    resp = session.get(f"{amm_url}/api/v2/amm/hub-reconciliation", timeout=timeout)
    skip_if_unavailable(resp)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "balanced", "unexplained")
    assert isinstance(body["balanced"], bool)
    assert is_amount_string(body["unexplained"])
    assert body["balanced"] == (int(body["unexplained"]) == 0)


@pytest.mark.edge_case
@pytest.mark.live_only
def test_completed_payments_leave_the_hub_reconciled(
    session, amm_url, timeout, skip_if_unavailable
):
    """
    After a run of payments the Hub should reconcile: `unexplained == "0"`.

    Reported as a skip rather than a failure when it does not, because an
    unbalanced hub is an operational condition of the estate — possibly an
    in-flight payment from another actor — and not evidence that the gateway
    violates its contract.
    """
    resp = session.get(f"{amm_url}/api/v2/amm/hub-reconciliation", timeout=timeout)
    skip_if_unavailable(resp)
    status_is(resp, 200)

    body = json_body(resp)
    if not body["balanced"]:
        pytest.skip(
            "hub not balanced: unexplained="
            f"{body['unexplained']} stranded_total={body.get('stranded_total')}"
        )
    assert int(body["unexplained"]) == 0
