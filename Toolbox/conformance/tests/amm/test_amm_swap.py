"""
Conformance: Scenario B cross-currency payment (quote + swap + residue).

Source contract: contracts/amm/openapi_amm_v2.3.0.yaml

The Scenario B payment is: reserve prelude -> bridge lock-mint -> Hub AMM swap ->
bridge-out -> residue return. This module covers the quote and swap legs and the
settlement verification that makes it a settlement test rather than an API test.

TWO SPEC-VS-IMPLEMENTATION DEFECTS ARE ENCODED HERE ON PURPOSE
  1. GET /api/v2/amm/quote/exact-output documents the parameter as `amountOut`;
     the handler reads `amount_out`. A client written strictly against the
     upstream document always gets 400. The suite sends `amount_out` and asserts
     the camelCase form is refused, so the divergence is evidenced, not hidden.
  2. GET /api/v2/amm/quote/cross-currency documents ZERO parameters yet requires
     source_currency, target_currency and amount_out. Following the upstream
     document literally always yields 400.

QUOTE TTL IS 15 SECONDS. Quote and swap must be issued programmatically in one
step; any pause between them flakes. Nothing here sleeps between the two.

THE RESIDUE MECHANIC, WHICH IS THE POINT
A cross-currency swap bridges in the FULL `max_amount_in` before the Hub swap
runs, because the realised input is unknown until execution. The unspent
remainder is burned on the Hub and returned to the payer on its source spoke as a
SEPARATE bridge position (leg RESIDUE). `residue_status = RETURN_FAILED` means
the payer has NOT been made whole. Asserting `status == COMPLETED` alone would
miss that entirely.

AUTH TRAP: the two swap endpoints are COOKIE-ONLY (RequireCookieAuth), not
RequireAnyAuth. A bearer token will not work on them, and no test here carries
`bearer_ok` except the one that asserts precisely that refusal.

ENVIRONMENT NEEDED FOR THE LIVE SETTLEMENT TESTS (they skip without it):
  CBWEB3_SOURCE_CURRENCY   e.g. BRL
  CBWEB3_TARGET_CURRENCY   e.g. ARS
  CBWEB3_BENEFICIARY_BANK_ID
  CBWEB3_SWAP_AMOUNT_OUT      optional, defaults to the contract's example
  CBWEB3_SWAP_MAX_AMOUNT_IN   optional, defaults to the contract's example
  CBWEB3_SWAP_POLL_SECONDS    optional, default 120
"""

import os
import time

import pytest

from tests.helpers import (
    SWAP_TERMINAL_STATES,
    V2_AMOUNT_OUT,
    V2_MAX_AMOUNT_IN,
    has_keys,
    is_amount_string,
    json_body,
    status_is,
)

pytestmark = [pytest.mark.amm, pytest.mark.amm_swap, pytest.mark.scenario_b]

QUOTE_TTL_SECONDS = 15


@pytest.fixture(scope="module")
def pool_pair(anon_session, amm_url, timeout):
    """Opaque `pair_id` from the registry. Never constructed by hand."""
    resp = anon_session.get(f"{amm_url}/api/v2/amm/pairs", timeout=timeout)
    if resp.status_code != 200:
        pytest.skip(f"GET /api/v2/amm/pairs -> {resp.status_code}")
    pairs = resp.json().get("pairs") or []
    if not pairs:
        pytest.skip("no AMM pair registered on this gateway")
    return pairs[0]["pair_id"]


@pytest.fixture(scope="module")
def corridor():
    """The currency corridor and beneficiary this deployment can actually settle."""
    source = os.environ.get("CBWEB3_SOURCE_CURRENCY")
    target = os.environ.get("CBWEB3_TARGET_CURRENCY")
    beneficiary = os.environ.get("CBWEB3_BENEFICIARY_BANK_ID")
    if not (source and target and beneficiary):
        pytest.skip(
            "set CBWEB3_SOURCE_CURRENCY, CBWEB3_TARGET_CURRENCY and "
            "CBWEB3_BENEFICIARY_BANK_ID to exercise a live cross-currency payment"
        )
    return {
        "source_currency": source,
        "target_currency": target,
        "beneficiary_bank_id": beneficiary,
        "amount_out": os.environ.get("CBWEB3_SWAP_AMOUNT_OUT", V2_AMOUNT_OUT),
        "max_amount_in": os.environ.get("CBWEB3_SWAP_MAX_AMOUNT_IN", V2_MAX_AMOUNT_IN),
    }


@pytest.fixture(scope="module")
def settled_swap(session, amm_url, timeout, pool_pair, corridor, require_profile):
    """
    Execute ONE real cross-currency payment and return its terminal status doc.

    Module-scoped so the three settlement assertions each read a different
    property of the same payment instead of moving money three times. The tests
    remain order-free: each depends only on this fixture, never on each other.
    """
    require_profile("scenario-b-bank", "scenario-b-cb")

    quote = session.get(
        f"{amm_url}/api/v2/amm/quote/cross-currency",
        params={
            "source_currency": corridor["source_currency"],
            "target_currency": corridor["target_currency"],
            "amount_out": corridor["amount_out"],
            "pool_pair": pool_pair,
        },
        timeout=timeout,
    )
    if quote.status_code == 422:
        pytest.skip(f"quote refused: {quote.text[:200]}")
    status_is(quote, 200)
    quote_id = quote.json().get("quote_id")

    # No pause here: the quote is valid for 15 seconds.
    swap = session.post(
        f"{amm_url}/api/v2/amm/swap/cross-currency",
        json={
            "source_currency": corridor["source_currency"],
            "target_currency": corridor["target_currency"],
            "pool_pair": pool_pair,
            "amount_out": corridor["amount_out"],
            "max_amount_in": corridor["max_amount_in"],
            "beneficiary_bank_id": corridor["beneficiary_bank_id"],
            **({"quote_id": quote_id} if quote_id else {}),
        },
        timeout=timeout,
    )
    if swap.status_code == 422:
        pytest.skip(f"swap refused: {swap.text[:200]}")
    status_is(swap, 200)
    swap_id = json_body(swap)["swap_id"]

    deadline = time.time() + int(os.environ.get("CBWEB3_SWAP_POLL_SECONDS", "120"))
    latest = {}
    while time.time() < deadline:
        poll = session.get(
            f"{amm_url}/api/v2/amm/swap/cross-currency/{swap_id}", timeout=timeout
        )
        status_is(poll, 200)
        latest = json_body(poll)
        if latest.get("status") in SWAP_TERMINAL_STATES:
            break
        time.sleep(3)

    assert latest.get("status") == "COMPLETED", (
        f"swap {swap_id} did not reach COMPLETED: status="
        f"{latest.get('status')} failure_reason={latest.get('failure_reason')}"
    )
    return latest


# ---------------------------------------------------------------------------
# Quotes — public
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_cross_currency_quote_is_public_and_expires_in_15_seconds(
    anon_session, amm_url, timeout, pool_pair
):
    """
    GET /api/v2/amm/quote/cross-currency -> 200.

    `created_at` and `valid_until` are UNIX SECONDS on this operation, not the
    RFC 3339 strings used by the record timestamps elsewhere in the same
    contract. The contract states `valid_until` is 15 seconds after `created_at`.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/quote/cross-currency",
        params={
            "source_currency": os.environ.get("CBWEB3_SOURCE_CURRENCY", "BRL"),
            "target_currency": os.environ.get("CBWEB3_TARGET_CURRENCY", "ARS"),
            "amount_out": V2_AMOUNT_OUT,
            "pool_pair": pool_pair,
        },
        timeout=timeout,
    )
    if resp.status_code == 422:
        pytest.skip(f"quote refused by this deployment: {resp.text[:200]}")

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "amount_in", "amount_out")
    assert is_amount_string(body["amount_in"])
    if "created_at" in body and "valid_until" in body:
        assert isinstance(body["created_at"], int)
        assert body["valid_until"] - body["created_at"] == QUOTE_TTL_SECONDS


@pytest.mark.error
@pytest.mark.live_only
def test_cross_currency_quote_without_parameters_returns_400(
    anon_session, amm_url, timeout, expect_error
):
    """
    The upstream document declares NO parameters on this operation, yet the
    handler requires three. Calling it the way the platform's own OpenAPI
    describes is always a 400 — evidenced here rather than quietly worked around.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/quote/cross-currency", timeout=timeout
    )

    expect_error(resp, 400)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_exact_output_quote_uses_the_snake_case_parameter(
    anon_session, amm_url, timeout, pool_pair
):
    """
    GET /api/v2/amm/quote/exact-output?pair=...&amount_out=... -> 200
    {required_input, price_impact, quote_timestamp}.

    `amount_out` is the name the handler reads. The upstream document says
    `amountOut`; the platform surface wins.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/quote/exact-output",
        params={"pair": pool_pair, "amount_out": V2_AMOUNT_OUT},
        timeout=timeout,
    )
    if resp.status_code == 422:
        pytest.skip(f"quote refused by this deployment: {resp.text[:200]}")

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "required_input")
    assert is_amount_string(body["required_input"])


@pytest.mark.error
@pytest.mark.live_only
def test_exact_output_quote_rejects_the_documented_camel_case_parameter(
    anon_session, amm_url, timeout, pool_pair, expect_error
):
    """
    Sending `amountOut` — the spelling the platform document publishes — is
    refused with 400. This test exists to keep the divergence provable.
    """
    resp = anon_session.get(
        f"{amm_url}/api/v2/amm/quote/exact-output",
        params={"pair": pool_pair, "amountOut": V2_AMOUNT_OUT},
        timeout=timeout,
    )

    expect_error(resp, 400)


# ---------------------------------------------------------------------------
# Swap — authentication
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.mock_safe
def test_cross_currency_swap_without_cookie_returns_401(
    anon_session, amm_url, timeout, pool_pair
):
    """
    The swap endpoints are cookie-only. The body is complete so the 401 cannot be
    mistaken for a validation failure.
    """
    resp = anon_session.post(
        f"{amm_url}/api/v2/amm/swap/cross-currency",
        json={
            "source_currency": "BRL",
            "target_currency": "ARS",
            "pool_pair": pool_pair,
            "amount_out": V2_AMOUNT_OUT,
            "max_amount_in": V2_MAX_AMOUNT_IN,
            "beneficiary_bank_id": "bank-galicia",
        },
        timeout=timeout,
    )

    status_is(resp, 401)


@pytest.mark.error
@pytest.mark.bearer_ok
@pytest.mark.live_only
def test_cross_currency_swap_refuses_a_bearer_token(
    session, amm_url, timeout, auth_mode, pool_pair
):
    """
    A bearer token is NOT accepted on the swap endpoints.

    BearerAuth is declared on exactly 12 Scenario B /api/v2 operations, all of
    them RequireAnyAuth; the two swap routes are RequireCookieAuth. Running with
    CBWEB3_AUTH_MODE=bearer must therefore still yield 401 here.
    """
    if auth_mode != "bearer":
        pytest.skip("bearer-mode assertion; run with CBWEB3_AUTH_MODE=bearer")

    resp = session.post(
        f"{amm_url}/api/v2/amm/swap/cross-currency",
        json={
            "source_currency": "BRL",
            "target_currency": "ARS",
            "pool_pair": pool_pair,
            "amount_out": V2_AMOUNT_OUT,
            "max_amount_in": V2_MAX_AMOUNT_IN,
            "beneficiary_bank_id": "bank-galicia",
        },
        timeout=timeout,
    )

    status_is(resp, 401)


@pytest.mark.error
@pytest.mark.live_only
def test_swap_validation_failure_carries_a_machine_readable_error_code(
    session, amm_url, timeout, pool_pair
):
    """
    The /api/v2 error envelope DOES have a machine-readable code, unlike /api/v1.

    AmmErrorResponse requires both `error` and `error_code` (observed vocabulary:
    INVALID_REQUEST, UNAUTHENTICATED, TRANSFER_LIMIT_EXCEEDED, POOL_NOT_ACTIVE,
    ...). Note the field is `error_code`, while the never-referenced
    ErrorCodeResponse schema in both platform specs calls it `code` — the suite
    asserts the name the live handlers actually emit.
    """
    resp = session.post(
        f"{amm_url}/api/v2/amm/swap/cross-currency",
        json={
            "source_currency": "BRL",
            "target_currency": "ARS",
            "pool_pair": pool_pair,
            "amount_out": "0",
            "max_amount_in": "0",
            "beneficiary_bank_id": "bank-galicia",
        },
        timeout=timeout,
    )

    status_is(resp, 400, 422)
    has_keys(json_body(resp), "error", "error_code")


# ---------------------------------------------------------------------------
# Swap — reads
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_swap_history_is_paginated_with_page_size_and_total(
    session, amm_url, timeout, skip_if_unavailable
):
    """
    GET /api/v2/amm/swap/cross-currency -> 200 {operations[], page, page_size, total}.

    This is the ONLY endpoint in either platform spec with a complete pagination
    envelope; audit logs echo page/limit without a total, and everything else is
    unpaginated. A 501 ("Swap history is not available on this gateway") skips.
    """
    resp = session.get(f"{amm_url}/api/v2/amm/swap/cross-currency", timeout=timeout)
    skip_if_unavailable(resp)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "operations", "page", "page_size", "total")
    assert isinstance(body["operations"], list)


@pytest.mark.error
@pytest.mark.live_only
def test_unknown_swap_id_returns_404(session, amm_url, timeout):
    """GET /api/v2/amm/swap/cross-currency/{id} declares 404 with an error_code."""
    resp = session.get(
        f"{amm_url}/api/v2/amm/swap/cross-currency/swp-does-not-exist", timeout=timeout
    )

    status_is(resp, 404)
    has_keys(json_body(resp), "error", "error_code")


# ---------------------------------------------------------------------------
# Settlement verification — the assertions that make this a settlement suite
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_completed_swap_consumed_amount_in_not_max_amount_in(settled_swap, corridor):
    """
    The payer's true debit is `amount_in`, which must not exceed `max_amount_in`.

    `max_amount_in` is only the slippage ceiling that gets bridged in; treating it
    as the cost is the single easiest way to misread this API.
    """
    has_keys(settled_swap, "amount_in", "amount_out")
    assert is_amount_string(settled_swap["amount_in"])
    assert int(settled_swap["amount_in"]) <= int(corridor["max_amount_in"])
    assert int(settled_swap["amount_out"]) > 0


@pytest.mark.happy_path
@pytest.mark.live_only
def test_completed_swap_returned_the_residue_to_the_payer(settled_swap):
    """
    The unspent slippage buffer must come back.

    `residue_status = RETURN_FAILED` means the payer has NOT been made whole and
    the buffer is stranded on the Hub swap signer — a COMPLETED swap can still
    leave the payer short. The platform documents no closed enum for this field,
    so only the failure value is asserted against.
    """
    residue_status = settled_swap.get("residue_status")
    if residue_status is None:
        pytest.skip("this deployment reported no residue_status for the swap")
    assert residue_status != "RETURN_FAILED", (
        "swap COMPLETED but the slippage buffer was not returned: "
        f"residue_amount={settled_swap.get('residue_amount')} "
        f"residue_position_id={settled_swap.get('residue_position_id')}"
    )


@pytest.mark.happy_path
@pytest.mark.live_only
def test_completed_swap_reports_both_bridge_legs(settled_swap):
    """
    A completed cross-currency payment names its bridge-in and bridge-out
    positions and the Hub transaction hash of the AMM leg. Without these three a
    caller cannot reconcile the payment against the bridge ledger.
    """
    has_keys(settled_swap, "bridge_in_position_id", "bridge_out_position_id")
    assert settled_swap["bridge_in_position_id"]
    assert settled_swap["bridge_out_position_id"]
    assert settled_swap.get("swap_tx_hash")
