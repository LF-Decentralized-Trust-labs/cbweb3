"""
Conformance: Scenario A FX agreement lifecycle.

Source contract: contracts/pvp/openapi_pvp_v2.3.0.yaml
  POST/GET  /api/v1/payments/fx/agreements
  GET       /api/v1/payments/fx/agreements/{tradeId}
  POST      .../accept .../reject .../cancel .../settle
  GET       .../audit

Scenario B has NO FX agreement surface whatsoever; every test here is
`scenario_a` and skips against a Scenario B profile.

GAPS NAMED, NOT FILLED
  * There is no EXPIRED state. `expiry_date` is on every agreement and the audit
    trail documents a SYSTEM_JOB "background expiration worker" event source, yet
    the state vocabulary is PROPOSED / ACCEPTED / REJECTED / CANCELLED / SETTLED.
    Expiry has no representable terminal state, so nothing is asserted about it.
  * FX_RATE_TOLERANCE_PCT (default 0.1%) governs whether a proposal is accepted,
    and FX_AGREEMENT_HTLC_STRICT governs HTLC linkage — neither is exposed by any
    endpoint, so a client cannot discover the server's configuration and the
    suite cannot assert against it.
  * Pente on/off materially changes FX behaviour (`group_id`,
    `contract_address`, and settle's "can also be called manually when Pente
    integration is not active") with no capability endpoint to distinguish
    deployments. `group_id` and `contract_address` are therefore never required.
  * The relay is documented to deliver FX state changes idempotently via the
    dedup key `${action}:${tradeId}`, but the only registered internal FX route
    is a GET. The write path is absent from the documented surface; nothing here
    models it.
"""

import time

import pytest

from tests.helpers import (
    FX_DEST_RECEIVER,
    FX_DEST_SPOKE_ID,
    FX_EVENT_SOURCES,
    FX_QUERY_STATES,
    FX_SOURCE_RECEIVER,
    FX_SOURCE_SPOKE_ID,
    FX_STATES,
    PALADIN_COUNTERPARTY_B,
    has_keys,
    is_amount_string,
    json_body,
    status_is,
)

pytestmark = [pytest.mark.pvp, pytest.mark.fx_agreement, pytest.mark.scenario_a]

# Example values taken verbatim from the contract's own ProposeFXAgreementRequest.
ORIGIN_AMOUNT = "1000000"
COUNTER_AMOUNT = "850000"
ORIGIN_CURRENCY = "BRL"
COUNTER_CURRENCY = "ARS"
RATE = "0.85"


def _proposal_body():
    """
    A minimally valid proposal: the ELEVEN required fields and nothing else.

    The gateway's own body check covers seven of them. The other four —
    `source_spoke_id`, `dest_spoke_id`, `source_receiver`, `dest_receiver` — are
    enforced one hop further in, by the orchestrator, which answers
    `InvalidArgument` and surfaces as 400. The platform document marks all four
    optional; they are not, and the contract now declares them required. Sending
    only the documented seven fails against every real deployment.

    `expiry_date` is a Unix timestamp in SECONDS and must be in the future — note
    it is an int64 epoch, unlike the record timestamps (`created_at` and friends)
    which are RFC 3339 strings. The two are never interchangeable.
    """
    return {
        "counterparty_b": PALADIN_COUNTERPARTY_B,
        "origin_amount": ORIGIN_AMOUNT,
        "counter_amount": COUNTER_AMOUNT,
        "origin_currency": ORIGIN_CURRENCY,
        "counter_currency": COUNTER_CURRENCY,
        "rate": RATE,
        "expiry_date": int(time.time()) + 86400,
        "source_spoke_id": FX_SOURCE_SPOKE_ID,
        "dest_spoke_id": FX_DEST_SPOKE_ID,
        "source_receiver": FX_SOURCE_RECEIVER,
        "dest_receiver": FX_DEST_RECEIVER,
    }


@pytest.fixture()
def trade_id(session, pvp_url, timeout, auth_mode):
    """
    A trade id to address path-parameter operations with.

    Against a live gateway a real agreement is proposed. Against Prism, which
    cannot persist one, the contract's own example id is used — Prism matches the
    path template, not the value.
    """
    if auth_mode == "mock":
        return "trade-a1b2c3d4"
    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements",
        json=_proposal_body(),
        timeout=timeout,
    )
    status_is(resp, 201)
    return json_body(resp)["trade_id"]


# ---------------------------------------------------------------------------
# Propose
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_propose_returns_201_with_trade_id(session, pvp_url, timeout):
    """POST /api/v1/payments/fx/agreements -> 201 {trade_id, tx_hash}."""
    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements",
        json=_proposal_body(),
        timeout=timeout,
    )

    status_is(resp, 201)
    body = json_body(resp)
    has_keys(body, "trade_id")
    assert isinstance(body["trade_id"], str) and body["trade_id"]


@pytest.mark.error
@pytest.mark.mock_safe
def test_propose_without_cookie_returns_401(anon_session, pvp_url, timeout, expect_error):
    """The FX surface has zero public operations."""
    resp = anon_session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements",
        json=_proposal_body(),
        timeout=timeout,
    )

    expect_error(resp, 401)


@pytest.mark.error
@pytest.mark.live_only
def test_propose_with_a_past_expiry_returns_400(session, pvp_url, timeout, expect_error):
    """
    `expiry_date must be a future Unix timestamp (seconds)` — the contract states
    it, and the operation declares 400 for an invalid body.
    """
    body = _proposal_body()
    body["expiry_date"] = int(time.time()) - 3600

    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements", json=body, timeout=timeout
    )

    expect_error(resp, 400)


@pytest.mark.error
@pytest.mark.live_only
def test_propose_with_identical_currencies_returns_400(session, pvp_url, timeout, expect_error):
    """`origin_currency` "must differ from counter_currency" — declared 400."""
    body = _proposal_body()
    body["counter_currency"] = body["origin_currency"]

    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements", json=body, timeout=timeout
    )

    expect_error(resp, 400)


# ---------------------------------------------------------------------------
# Read
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_agreements_returns_agreements_and_total(session, pvp_url, timeout):
    """GET /api/v1/payments/fx/agreements -> 200 {agreements[], total}."""
    resp = session.get(f"{pvp_url}/api/v1/payments/fx/agreements", timeout=timeout)

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "agreements", "total")
    assert isinstance(body["agreements"], list)
    assert isinstance(body["total"], int)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_list_agreements_accepts_the_declared_state_filter(session, pvp_url, timeout):
    """
    The `state` query parameter takes the BARE vocabulary (`PROPOSED`, ...), while
    every returned agreement's `state` uses the PREFIXED one (`FX_STATE_PROPOSED`,
    ...). Two vocabularies, one endpoint: the filter is matched after the delivered
    orchestrator strips an `FX_STATE_` prefix, so both spellings are accepted on
    input, but the output is always prefixed.

    Membership in FX_STATES is also the assertion that records the absence of an
    EXPIRED state.
    """
    assert "PROPOSED" in FX_QUERY_STATES
    resp = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements",
        params={"state": "PROPOSED"},
        timeout=timeout,
    )

    status_is(resp, 200)
    for agreement in json_body(resp)["agreements"]:
        if "state" in agreement:
            assert agreement["state"] in FX_STATES


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_get_agreement_returns_the_agreement_object(session, pvp_url, timeout, trade_id):
    """
    GET /api/v1/payments/fx/agreements/{tradeId} -> 200 {agreement}.

    Amounts and the rate are strings. The rate is the one string that carries a
    fractional point ("0.85"); amounts are base-10 integers.
    """
    resp = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}", timeout=timeout
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "agreement")
    agreement = body["agreement"]
    if "state" in agreement:
        assert agreement["state"] in FX_STATES
    for field in ("origin_amount", "counter_amount", "rate"):
        if field in agreement:
            assert is_amount_string(agreement[field])
    if "expiry_date" in agreement:
        # int64 Unix seconds, NOT an RFC 3339 string like `created_at`.
        assert isinstance(agreement["expiry_date"], int)


@pytest.mark.error
@pytest.mark.live_only
def test_get_unknown_agreement_returns_404(session, pvp_url, timeout, expect_error):
    """The operation declares 404. Prism would answer its static 200 example."""
    resp = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/trade-does-not-exist",
        timeout=timeout,
    )

    expect_error(resp, 404)


@pytest.mark.happy_path
@pytest.mark.mock_safe
def test_audit_trail_returns_events_and_total(session, pvp_url, timeout, trade_id):
    """
    GET .../{tradeId}/audit -> 200 {events[], total}.

    Each event carries from_state/to_state drawn from the same PREFIXED
    vocabulary as `FXAgreement.state` (`FX_STATE_*`) and a `source` of
    `FX_EVENT_SOURCE_LOCAL_API | _RELAY | _SYSTEM_JOB | _ON_BEHALF`. The platform
    document declares both without the prefix; the delivered gateway serialises
    the protobuf enum names, so the prefixed set is what actually arrives.
    `occurred_at_unix` is epoch seconds, not RFC 3339.
    """
    resp = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/audit", timeout=timeout
    )

    status_is(resp, 200)
    body = json_body(resp)
    has_keys(body, "events", "total")
    assert isinstance(body["events"], list)
    for event in body["events"]:
        for field in ("from_state", "to_state"):
            if field in event:
                assert event[field] in FX_STATES
        if "source" in event:
            assert event["source"] in FX_EVENT_SOURCES
        if "occurred_at_unix" in event:
            assert isinstance(event["occurred_at_unix"], int)


# ---------------------------------------------------------------------------
# Transitions
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.live_only
def test_accept_moves_a_proposed_agreement_to_accepted(
    session, pvp_url, timeout, trade_id, require_profile
):
    """
    POST .../{tradeId}/accept -> 200 {tx_hash}, and the agreement reads
    FX_STATE_ACCEPTED.

    Acceptance belongs to `counterparty_b`. A single-credential run can only
    perform it from a governance session using `on_behalf`, which is why this
    test requires the Central Bank profile; a commercial-bank run would need the
    counterparty's own session and is out of scope for a one-credential suite.
    """
    require_profile("scenario-a-cb")

    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/accept",
        json={"on_behalf": True},
        timeout=timeout,
    )

    status_is(resp, 200)
    has_keys(json_body(resp), "tx_hash")

    read = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}", timeout=timeout
    )
    status_is(read, 200)
    assert json_body(read)["agreement"]["state"] == "FX_STATE_ACCEPTED"


@pytest.mark.happy_path
@pytest.mark.live_only
def test_reject_is_a_terminal_branch(session, pvp_url, timeout, trade_id, require_profile):
    """POST .../{tradeId}/reject -> 200, state FX_STATE_REJECTED."""
    require_profile("scenario-a-cb")

    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/reject",
        json={"on_behalf": True},
        timeout=timeout,
    )

    status_is(resp, 200)
    read = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}", timeout=timeout
    )
    assert json_body(read)["agreement"]["state"] == "FX_STATE_REJECTED"


@pytest.mark.happy_path
@pytest.mark.live_only
def test_cancel_is_a_terminal_branch(session, pvp_url, timeout, trade_id):
    """
    POST .../{tradeId}/cancel -> 200, state FX_STATE_CANCELLED. Cancellation belongs to the
    originator, so it needs no `on_behalf` and no Central Bank profile. The
    operation takes no request body at all.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/cancel", timeout=timeout
    )

    status_is(resp, 200)
    read = session.get(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}", timeout=timeout
    )
    assert json_body(read)["agreement"]["state"] == "FX_STATE_CANCELLED"


@pytest.mark.error
@pytest.mark.live_only
def test_settling_a_proposed_agreement_returns_409(
    session, pvp_url, timeout, trade_id, expect_error
):
    """
    Settlement of an agreement that is not FX_STATE_ACCEPTED is refused by the
    state machine.

    `Divergence:` the platform OpenAPI document declares 412 for this refusal,
    but no payments or HTLC route in the delivered gateway can emit 412 — the
    orchestrator returns gRPC FailedPrecondition and `grpcErrorToHTTP` maps it to
    409 Conflict. The contract, the mocks (pvp/errors/02_invalid_state_transition)
    and the vectors (fx-err-07/08/09) all say 409, so this test does too.

    No machine-readable error code exists to assert on — the /api/v1 error body is
    `{error: "<free-form string>"}` and nothing more.
    """
    resp = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/settle", timeout=timeout
    )

    expect_error(resp, 409)


@pytest.mark.edge_case
@pytest.mark.live_only
def test_accepting_twice_returns_409(session, pvp_url, timeout, trade_id, require_profile):
    """
    The second accept violates the state precondition. The contract declares
    400/401/404/409/500 for this operation; 409 Conflict is the state-machine
    refusal the delivered gateway returns (the platform document's 412 is
    unreachable — see test_settling_a_proposed_agreement_returns_409).

    There is no idempotency mechanism anywhere in this API — no Idempotency-Key,
    no If-Match/ETag — so a replayed transition is a real state error, not a
    no-op.
    """
    require_profile("scenario-a-cb")

    first = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/accept",
        json={"on_behalf": True},
        timeout=timeout,
    )
    status_is(first, 200)

    second = session.post(
        f"{pvp_url}/api/v1/payments/fx/agreements/{trade_id}/accept",
        json={"on_behalf": True},
        timeout=timeout,
    )
    status_is(second, 409)
