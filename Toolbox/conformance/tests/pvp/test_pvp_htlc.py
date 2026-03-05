"""
Conformance tests: PvP Settlement — HTLC

Source contract: contracts/pvp/openapi_pvp_v0.1.0.yaml
Source vectors:  test-vectors/pvp/pvp_htlc_vectors.json

These tests validate that an implementation of the CBWeb3 PvP Settlement API
conforms to the HTLC interface contract.

Synthetic cryptographic data:
  Secret:   cbweb3-test-secret-2026
  SHA-256:  0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069
"""

import time

import pytest
import requests

VALID_HASH_LOCK = (
    "0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069"
)
VALID_SECRET = "cbweb3-test-secret-2026"
INVALID_SECRET = "wrong-secret"

# timeLock far in the future (year 2030) for happy-path tests
FUTURE_TIME_LOCK = 1893456000

# timeLock in the past for expiry tests
PAST_TIME_LOCK = 1700000000


@pytest.fixture()
def accepted_agreement(base_url, auth_headers):
    """Create and accept an FX agreement, return the agreementId."""
    create_resp = requests.post(
        f"{base_url}/api/v1/fx/agreement",
        json={
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        },
        headers=auth_headers,
    )
    assert create_resp.status_code == 201
    agreement_id = create_resp.json()["agreementId"]

    accept_resp = requests.post(
        f"{base_url}/api/v1/fx/agreement/{agreement_id}/accept",
        headers=auth_headers,
    )
    assert accept_resp.status_code == 200

    return agreement_id


@pytest.fixture()
def proposed_agreement(base_url, auth_headers):
    """Create an FX agreement but do NOT accept it. Return the agreementId."""
    create_resp = requests.post(
        f"{base_url}/api/v1/fx/agreement",
        json={
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        },
        headers=auth_headers,
    )
    assert create_resp.status_code == 201
    return create_resp.json()["agreementId"]


@pytest.fixture()
def locked_htlc(base_url, auth_headers, accepted_agreement):
    """Lock funds for an accepted agreement, return (agreementId, contractId)."""
    lock_resp = requests.post(
        f"{base_url}/api/v1/htlc/lock",
        json={
            "agreementId": accepted_agreement,
            "hashLock": VALID_HASH_LOCK,
            "timeLock": FUTURE_TIME_LOCK,
        },
        headers=auth_headers,
    )
    assert lock_resp.status_code == 201
    contract_id = lock_resp.json()["contractId"]
    return accepted_agreement, contract_id


# ---------------------------------------------------------------------------
# Vector htlc-hp-01: Lock funds — happy path
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.htlc
class TestHtlcLock:
    """POST /htlc/lock — lock tCeBM linked to an accepted FX agreement."""

    def test_returns_201_with_locked_status(
        self, base_url, auth_headers, accepted_agreement
    ):
        """Locking funds for a READY_FOR_SETTLEMENT agreement returns 201 LOCKED."""
        resp = requests.post(
            f"{base_url}/api/v1/htlc/lock",
            json={
                "agreementId": accepted_agreement,
                "hashLock": VALID_HASH_LOCK,
                "timeLock": FUTURE_TIME_LOCK,
            },
            headers=auth_headers,
        )

        assert resp.status_code == 201
        body = resp.json()
        assert "contractId" in body
        assert isinstance(body["contractId"], str) and len(body["contractId"]) > 0
        assert "transactionHash" in body
        assert isinstance(body["transactionHash"], str) and len(body["transactionHash"]) > 0
        assert body["status"] == "LOCKED"
        assert "blockNumber" in body
        assert isinstance(body["blockNumber"], int) and body["blockNumber"] > 0


# ---------------------------------------------------------------------------
# Vector htlc-hp-02: Settle HTLC — valid secret
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.htlc
class TestHtlcSettle:
    """POST /htlc/settle — reveal secret to claim locked funds."""

    def test_returns_200_with_settled_status(
        self, base_url, auth_headers, locked_htlc
    ):
        """Revealing the correct secret settles the HTLC (200 SETTLED)."""
        _, contract_id = locked_htlc

        resp = requests.post(
            f"{base_url}/api/v1/htlc/settle",
            json={
                "contractId": contract_id,
                "secret": VALID_SECRET,
            },
            headers=auth_headers,
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["contractId"] == contract_id
        assert body["status"] == "SETTLED"


# ---------------------------------------------------------------------------
# Vector htlc-err-01: Settle HTLC — invalid secret (hash mismatch)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.htlc
class TestHtlcSettleInvalidSecret:
    """POST /htlc/settle with wrong secret returns 400 HTLC_HASH_MISMATCH."""

    def test_returns_400_hash_mismatch(self, base_url, auth_headers, locked_htlc):
        """Providing a secret that does not hash to the hashLock returns 400."""
        _, contract_id = locked_htlc

        resp = requests.post(
            f"{base_url}/api/v1/htlc/settle",
            json={
                "contractId": contract_id,
                "secret": INVALID_SECRET,
            },
            headers=auth_headers,
        )

        assert resp.status_code == 400
        body = resp.json()
        assert body["code"] == "HTLC_HASH_MISMATCH"
        assert "message" in body
        assert isinstance(body["message"], str) and len(body["message"]) > 0


# ---------------------------------------------------------------------------
# Vector htlc-err-02: Settle HTLC — expired timeLock (410)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.htlc
class TestHtlcSettleExpired:
    """POST /htlc/settle after timeLock expiry returns 410."""

    def test_returns_410_expired(self, base_url, auth_headers, accepted_agreement):
        """Settling an expired HTLC returns 410 with HTLC_EXPIRED error."""
        # Lock with a timeLock in the past
        lock_resp = requests.post(
            f"{base_url}/api/v1/htlc/lock",
            json={
                "agreementId": accepted_agreement,
                "hashLock": VALID_HASH_LOCK,
                "timeLock": PAST_TIME_LOCK,
            },
            headers=auth_headers,
        )
        # Note: some implementations may reject locking with a past timeLock (400).
        # If lock succeeds, attempt settle should return 410.
        if lock_resp.status_code != 201:
            pytest.skip("Implementation rejects lock with past timeLock (acceptable)")

        contract_id = lock_resp.json()["contractId"]

        resp = requests.post(
            f"{base_url}/api/v1/htlc/settle",
            json={
                "contractId": contract_id,
                "secret": VALID_SECRET,
            },
            headers=auth_headers,
        )

        assert resp.status_code == 410
        body = resp.json()
        assert body["code"] == "HTLC_EXPIRED"


# ---------------------------------------------------------------------------
# Vector htlc-hp-03: Refund HTLC — after timeLock expiry
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.htlc
class TestHtlcRefund:
    """POST /htlc/refund — reclaim funds after timeLock expires."""

    def test_returns_200_with_refunded_status(
        self, base_url, auth_headers, accepted_agreement
    ):
        """Refunding an expired, unsettled HTLC returns 200 REFUNDED."""
        # Lock with a timeLock in the past
        lock_resp = requests.post(
            f"{base_url}/api/v1/htlc/lock",
            json={
                "agreementId": accepted_agreement,
                "hashLock": VALID_HASH_LOCK,
                "timeLock": PAST_TIME_LOCK,
            },
            headers=auth_headers,
        )
        if lock_resp.status_code != 201:
            pytest.skip("Implementation rejects lock with past timeLock (acceptable)")

        contract_id = lock_resp.json()["contractId"]

        resp = requests.post(
            f"{base_url}/api/v1/htlc/refund",
            json={"contractId": contract_id},
            headers=auth_headers,
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["contractId"] == contract_id
        assert body["status"] == "REFUNDED"


# ---------------------------------------------------------------------------
# Vector htlc-err-03: Refund HTLC — not yet expired (409)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.htlc
class TestHtlcRefundNotExpired:
    """POST /htlc/refund before timeLock expiry returns 409."""

    def test_returns_409(self, base_url, auth_headers, locked_htlc):
        """Attempting refund before timeLock expires returns 409."""
        _, contract_id = locked_htlc

        resp = requests.post(
            f"{base_url}/api/v1/htlc/refund",
            json={"contractId": contract_id},
            headers=auth_headers,
        )

        assert resp.status_code == 409


# ---------------------------------------------------------------------------
# Vector htlc-err-04: Refund HTLC — already settled (409)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.htlc
class TestHtlcRefundAlreadySettled:
    """POST /htlc/refund on a settled HTLC returns 409."""

    def test_returns_409(self, base_url, auth_headers, locked_htlc):
        """Refunding an already-settled HTLC returns 409."""
        _, contract_id = locked_htlc

        # Settle it first
        settle_resp = requests.post(
            f"{base_url}/api/v1/htlc/settle",
            json={
                "contractId": contract_id,
                "secret": VALID_SECRET,
            },
            headers=auth_headers,
        )
        assert settle_resp.status_code == 200

        # Try to refund
        resp = requests.post(
            f"{base_url}/api/v1/htlc/refund",
            json={"contractId": contract_id},
            headers=auth_headers,
        )

        assert resp.status_code == 409


# ---------------------------------------------------------------------------
# Vector htlc-edge-01: Lock HTLC — agreement not yet accepted
# ---------------------------------------------------------------------------
@pytest.mark.edge_case
@pytest.mark.htlc
class TestHtlcLockNotAccepted:
    """POST /htlc/lock for a PROPOSED (not accepted) agreement returns 400."""

    def test_returns_400(self, base_url, auth_headers, proposed_agreement):
        """Lock should only be allowed when agreement is READY_FOR_SETTLEMENT."""
        resp = requests.post(
            f"{base_url}/api/v1/htlc/lock",
            json={
                "agreementId": proposed_agreement,
                "hashLock": VALID_HASH_LOCK,
                "timeLock": FUTURE_TIME_LOCK,
            },
            headers=auth_headers,
        )

        assert resp.status_code == 400


# ---------------------------------------------------------------------------
# HTLC Status — happy path
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.htlc
class TestHtlcStatus:
    """GET /htlc/status — check HTLC contract state."""

    def test_returns_200_with_status(self, base_url, auth_headers, locked_htlc):
        """Querying an existing HTLC returns 200 with HtlcStatusResponse schema."""
        _, contract_id = locked_htlc

        resp = requests.get(
            f"{base_url}/api/v1/htlc/status",
            params={"contractId": contract_id},
            headers=auth_headers,
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["contractId"] == contract_id
        assert body["status"] in ["LOCKED", "SETTLED", "REFUNDED", "EXPIRED"]
        assert "hashLock" in body
        assert "timeLock" in body
        assert "amount" in body
        assert "sender" in body
        assert "receiver" in body
