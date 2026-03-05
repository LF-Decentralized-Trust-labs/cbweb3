"""
Conformance tests: PvP Settlement — FX Agreement

Source contract: contracts/pvp/openapi_pvp_v0.1.0.yaml
Source vectors:  test-vectors/pvp/pvp_fx_agreement_vectors.json

These tests validate that an implementation of the CBWeb3 PvP Settlement API
conforms to the FX Agreement interface contract.
"""

import pytest
import requests


# ---------------------------------------------------------------------------
# Vector fx-hp-01: Create FX Agreement — happy path
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.fx_agreement
class TestCreateFxAgreement:
    """POST /fx/agreement — create a new FX agreement proposal."""

    def test_returns_201(self, base_url, auth_headers):
        """A valid FX agreement proposal returns 201."""
        payload = {
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        }
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement",
            json=payload,
            headers=auth_headers,
        )

        assert resp.status_code == 201
        body = resp.json()
        assert "agreementId" in body, "Response must contain agreementId"
        assert isinstance(body["agreementId"], str)
        assert len(body["agreementId"]) > 0, "agreementId must be non-empty"
        assert body["status"] == "PROPOSED"

    def test_response_matches_schema(self, base_url, auth_headers):
        """Response body matches FxAgreementResponse schema."""
        payload = {
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "500000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        }
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement",
            json=payload,
            headers=auth_headers,
        )

        assert resp.status_code == 201
        body = resp.json()
        assert isinstance(body.get("agreementId"), str)
        assert body.get("status") in [
            "PROPOSED",
            "READY_FOR_SETTLEMENT",
            "SETTLED",
            "CANCELLED",
            "EXPIRED",
        ]


# ---------------------------------------------------------------------------
# Vector fx-err-01: Create FX Agreement — missing required field
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.fx_agreement
class TestCreateFxAgreementMissingField:
    """POST /fx/agreement with missing required field returns 400."""

    def test_missing_exchange_rate(self, base_url, auth_headers):
        """Request without exchangeRate returns 400 with Error schema."""
        payload = {
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
            # exchangeRate intentionally omitted
        }
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement",
            json=payload,
            headers=auth_headers,
        )

        assert resp.status_code == 400
        body = resp.json()
        assert "code" in body, "Error response must contain code"
        assert "message" in body, "Error response must contain message"
        assert isinstance(body["code"], str) and len(body["code"]) > 0
        assert isinstance(body["message"], str) and len(body["message"]) > 0

    def test_missing_source_currency(self, base_url, auth_headers):
        """Request without sourceCurrency returns 400."""
        payload = {
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        }
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement",
            json=payload,
            headers=auth_headers,
        )

        assert resp.status_code == 400


# ---------------------------------------------------------------------------
# Vector fx-hp-02: Accept FX Agreement — happy path
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.fx_agreement
class TestAcceptFxAgreement:
    """POST /fx/agreement/{id}/accept — counterparty accepts."""

    def test_returns_200_with_ready_status(self, base_url, auth_headers):
        """Accepting a PROPOSED agreement returns 200 with READY_FOR_SETTLEMENT."""
        # First create an agreement to accept
        create_payload = {
            "sourceCurrency": "tCeBM-CRC",
            "targetCurrency": "tCeBM-DOP",
            "sourceAmount": "1000000",
            "exchangeRate": "0.058",
            "counterpartyAddress": "0xCB002_DominicanRepublic",
        }
        create_resp = requests.post(
            f"{base_url}/api/v1/fx/agreement",
            json=create_payload,
            headers=auth_headers,
        )
        assert create_resp.status_code == 201
        agreement_id = create_resp.json()["agreementId"]

        # Accept it
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement/{agreement_id}/accept",
            headers=auth_headers,
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["agreementId"] == agreement_id
        assert body["status"] == "READY_FOR_SETTLEMENT"


# ---------------------------------------------------------------------------
# Vector fx-err-02: Accept FX Agreement — already accepted (409)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.fx_agreement
class TestAcceptFxAgreementAlreadyAccepted:
    """POST /fx/agreement/{id}/accept on already-accepted agreement returns 409."""

    def test_double_accept_returns_409(self, base_url, auth_headers):
        """Accepting an agreement that is already READY_FOR_SETTLEMENT returns 409."""
        # Create and accept
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

        # Try to accept again
        resp = requests.post(
            f"{base_url}/api/v1/fx/agreement/{agreement_id}/accept",
            headers=auth_headers,
        )

        assert resp.status_code == 409


# ---------------------------------------------------------------------------
# Vector fx-err-03: Get non-existent agreement (404)
# ---------------------------------------------------------------------------
@pytest.mark.error
@pytest.mark.fx_agreement
class TestGetNonExistentAgreement:
    """GET /fx/agreement/{id} for non-existent agreement returns 404."""

    def test_returns_404(self, base_url, auth_headers):
        """Querying a non-existent agreement returns 404."""
        resp = requests.get(
            f"{base_url}/api/v1/fx/agreement/agr-nonexistent-999",
            headers=auth_headers,
        )

        assert resp.status_code == 404


# ---------------------------------------------------------------------------
# Get agreement details — happy path (schema check)
# ---------------------------------------------------------------------------
@pytest.mark.happy_path
@pytest.mark.fx_agreement
class TestGetFxAgreementDetails:
    """GET /fx/agreement/{id} — returns full agreement details."""

    def test_returns_200_with_details(self, base_url, auth_headers):
        """Querying an existing agreement returns 200 with FxAgreementDetails schema."""
        # Create an agreement first
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

        # Query it
        resp = requests.get(
            f"{base_url}/api/v1/fx/agreement/{agreement_id}",
            headers=auth_headers,
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["agreementId"] == agreement_id
        assert "status" in body
