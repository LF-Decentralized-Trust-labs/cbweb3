"""
Conformance: Scenario B reserve lifecycle.

Source contract: contracts/amm/openapi_amm_v2.3.0.yaml

The reserve lifecycle is deliberately duplicated between the `pvp` and `amm`
contracts because the two gateways genuinely differ. The differences these tests
encode, none of which may be normalised away:

  * Fiat conversion:  Scenario B  POST /api/v1/payments/deposits/exchange
                      Scenario A  POST /api/v1/payments/deposits/fiat-exchange
  * Its 201 body:     Scenario B  {mint_tx_hash}      Scenario A  {tx_hash}
  * Escrow approval:  Scenario B  {burn_tx_hash, mint_tx_hash}
                      Scenario A  {burn_tx_hash, zeto_mint_tx_hash}
  * Escrow request:   Scenario B accepts an optional `deposit_id`;
                      Scenario A has no such field but does carry
                      `requester_paladin_identity`.
  * Redeem request:   Scenario B requires only `amount`;
                      Scenario A declares `zeto_transfer_tx_hash`, but its proxy injects it.

Reserve Tokenisation is MANDATORY before any Scenario B payment: a bank cannot
bridge or swap tokens it does not hold.
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

pytestmark = [pytest.mark.amm, pytest.mark.reserves, pytest.mark.scenario_b]


@pytest.fixture()
def registered_deposit(session, amm_url, timeout):
    resp = session.post(
        f"{amm_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )
    status_is(resp, 201)
    return json_body(resp)["deposit_id"]


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_register_deposit_returns_201_with_deposit_id(session, amm_url, timeout):
    """
    POST /api/v1/payments/deposits -> 201 {deposit_id}.

    Scenario B's response declares `deposit_id` only — Scenario A's also declares
    `status`. Never send `requester_besu_address`: the commercial-bank proxy
    injects it from the verified caller and overwrites anything the client sends.
    """
    resp = session.post(
        f"{amm_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )

    status_is(resp, 201)
    has_keys(json_body(resp), "deposit_id")


@pytest.mark.error
@pytest.mark.mock_safe
def test_register_deposit_without_cookie_returns_401(
    anon_session, amm_url, timeout, expect_error
):
    """The whole /api/v1 surface is CookieAuth on Scenario B too."""
    resp = anon_session.post(
        f"{amm_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )

    expect_error(resp, 401)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_deposits_returns_deposits_and_total(session, amm_url, timeout):
    """GET /api/v1/payments/deposits -> 200 {deposits[], total}."""
    resp = session.get(f"{amm_url}/api/v1/payments/deposits", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "deposits", "total")
    assert isinstance(body["deposits"], list)
    for record in body["deposits"]:
        if "status" in record:
            assert record["status"] in RESERVE_STATES


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_request_escrow_returns_201_with_escrow_id(session, amm_url, timeout):
    """POST /api/v1/payments/escrows -> 201 {escrow_id}."""
    resp = session.post(
        f"{amm_url}/api/v1/payments/escrows",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )

    status_is(resp, 201)
    has_keys(json_body(resp), "escrow_id")


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_escrows_returns_escrows_and_total(session, amm_url, timeout):
    """GET /api/v1/payments/escrows -> 200 {escrows[], total}."""
    resp = session.get(f"{amm_url}/api/v1/payments/escrows", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "escrows", "total")
    assert isinstance(body["escrows"], list)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_request_redeem_returns_201_with_redeem_id(session, amm_url, timeout):
    """
    POST /api/v1/payments/redeems -> 201 {redeem_id}.

    Scenario B requires `amount` only — and so, in practice, does Scenario A: its
    `zeto_transfer_tx_hash` is injected by the commercial-bank proxy, not supplied
    by the caller. The Scenario A equivalent is live-only because its proxy moves
    real value before forwarding, not because of an extra field.
    """
    resp = session.post(
        f"{amm_url}/api/v1/payments/redeems",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )

    status_is(resp, 201)
    has_keys(json_body(resp), "redeem_id")


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_token_balance_is_a_string(session, amm_url, timeout):
    """GET /api/v1/token/balance -> 200 {balance}, a decimal string."""
    resp = session.get(f"{amm_url}/api/v1/token/balance", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "balance")
    assert is_amount_string(body["balance"])


@pytest.mark.happy_path
@pytest.mark.live_only
def test_approve_escrow_returns_burn_and_mint_hashes(
    session, amm_url, timeout, require_profile, skip_if_unavailable
):
    """
    POST /api/v1/payments/escrows/approve -> 201 {burn_tx_hash, mint_tx_hash}.

    Note `mint_tx_hash`, not Scenario A's `zeto_mint_tx_hash`. Central Bank
    gateways only.
    """
    require_profile("scenario-b-cb")

    escrow = session.post(
        f"{amm_url}/api/v1/payments/escrows",
        json={"amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )
    status_is(escrow, 201)

    resp = session.post(
        f"{amm_url}/api/v1/payments/escrows/approve",
        json={"escrow_id": json_body(escrow)["escrow_id"]},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 201)
    has_keys(json_body(resp), "burn_tx_hash", "mint_tx_hash")


@pytest.mark.happy_path
@pytest.mark.live_only
def test_deposit_exchange_returns_a_mint_tx_hash(
    session, amm_url, timeout, registered_deposit, require_profile, skip_if_unavailable
):
    """
    POST /api/v1/payments/deposits/exchange -> 201 {mint_tx_hash}.

    The Scenario B spelling. Calling Scenario A's `/deposits/fiat-exchange`
    against a Scenario B gateway is a 404, and vice versa.
    """
    require_profile("scenario-b-cb")

    approved = session.post(
        f"{amm_url}/api/v1/payments/deposits/approve",
        json={"deposit_id": registered_deposit},
        timeout=timeout,
    )
    skip_if_unavailable(approved)
    status_is(approved, 201)

    resp = session.post(
        f"{amm_url}/api/v1/payments/deposits/exchange",
        json={"deposit_id": registered_deposit},
        timeout=timeout,
    )
    skip_if_unavailable(resp)

    status_is(resp, 201)
    has_keys(json_body(resp), "mint_tx_hash")


@pytest.mark.happy_path
@pytest.mark.live_only
def test_reserve_prelude_yields_a_non_zero_tcebm_balance(
    session, amm_url, timeout, require_profile, skip_if_unavailable
):
    """deposit -> approve -> escrow -> approve -> balance > 0, the Scenario B prelude."""
    require_profile("scenario-b-cb")

    deposit = session.post(
        f"{amm_url}/api/v1/payments/deposits",
        json={"amount": V1_DEPOSIT_AMOUNT},
        timeout=timeout,
    )
    status_is(deposit, 201)
    deposit_id = json_body(deposit)["deposit_id"]

    approved = session.post(
        f"{amm_url}/api/v1/payments/deposits/approve",
        json={"deposit_id": deposit_id},
        timeout=timeout,
    )
    skip_if_unavailable(approved)
    status_is(approved, 201)

    escrow = session.post(
        f"{amm_url}/api/v1/payments/escrows",
        json={"deposit_id": deposit_id, "amount": V1_ESCROW_AMOUNT},
        timeout=timeout,
    )
    status_is(escrow, 201)

    tokenised = session.post(
        f"{amm_url}/api/v1/payments/escrows/approve",
        json={"escrow_id": json_body(escrow)["escrow_id"]},
        timeout=timeout,
    )
    skip_if_unavailable(tokenised)
    status_is(tokenised, 201)

    balance = session.get(f"{amm_url}/api/v1/token/balance", timeout=timeout)
    status_is(balance, 200)
    assert int(json_body(balance)["balance"]) > 0
